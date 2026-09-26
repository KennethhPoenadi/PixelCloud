"""Process a single job: download, run the pipeline, upload, record the result."""

from __future__ import annotations

import io
import signal
import threading
import time
import warnings
from collections.abc import Iterator
from contextlib import contextmanager
from datetime import UTC, datetime
from types import FrameType

import structlog
from PIL import Image, UnidentifiedImageError

from worker import metrics, pipeline
from worker.config import Config
from worker.db import ClaimedJob, Database
from worker.queue import JobQueue, Message
from worker.storage import Storage

HEARTBEAT_SECONDS = 5.0


class JobTimeoutError(Exception):
    pass


class PermanentError(Exception):
    """Retrying will not help (bad pipeline, corrupt or oversized image)."""


@contextmanager
def time_limit(seconds: int) -> Iterator[None]:
    """Raise JobTimeoutError in the main thread after `seconds`."""

    def on_alarm(_signum: int, _frame: FrameType | None) -> None:
        raise JobTimeoutError(f"job exceeded {seconds}s time limit")

    previous = signal.signal(signal.SIGALRM, on_alarm)
    signal.setitimer(signal.ITIMER_REAL, seconds)
    try:
        yield
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, previous)


@contextmanager
def heartbeat(queue: JobQueue, entry_id: str, log: structlog.stdlib.BoundLogger) -> Iterator[None]:
    """Keep our queue entry fresh so other workers do not reclaim it."""
    done = threading.Event()

    def beat() -> None:
        while not done.wait(HEARTBEAT_SECONDS):
            try:
                queue.heartbeat(entry_id)
            except Exception as exc:
                log.warning("heartbeat failed", error=str(exc))

    thread = threading.Thread(target=beat, name="job-heartbeat", daemon=True)
    thread.start()
    try:
        yield
    finally:
        done.set()
        thread.join(timeout=1)


class Processor:
    def __init__(
        self,
        cfg: Config,
        db: Database,
        storage: Storage,
        queue: JobQueue,
        log: structlog.stdlib.BoundLogger,
    ) -> None:
        self._cfg = cfg
        self._db = db
        self._storage = storage
        self._queue = queue
        self._log = log
        Image.MAX_IMAGE_PIXELS = cfg.max_image_pixels
        warnings.simplefilter("error", Image.DecompressionBombWarning)

    def handle(self, msg: Message) -> None:
        node = self._cfg.node_id
        structlog.contextvars.bind_contextvars(job_id=msg.job_id, request_id=msg.request_id)
        try:
            job = self._db.claim(msg.job_id, node)
            if job is None:
                # already finished, or the image was deleted meanwhile
                self._log.info("job no longer claimable, skipping")
                self._queue.ack(msg.entry_id)
                return
            if msg.reclaimed:
                metrics.JOBS_RECLAIMED.labels(node=node).inc()
                self._log.warning("reclaimed job from unresponsive worker", attempts=job.attempts)
            if job.attempts > self._cfg.job_max_attempts:
                self._db.fail(job.id, node, f"gave up after {self._cfg.job_max_attempts} attempts")
                metrics.JOBS_PROCESSED.labels(node=node, status="failed").inc()
                self._queue.ack(msg.entry_id)
                return
            if job.attempts == 1:
                wait = (datetime.now(UTC) - job.created_at).total_seconds()
                metrics.JOB_QUEUE_WAIT_SECONDS.labels(node=node).observe(max(wait, 0.0))

            self._process(msg, job)
            self._queue.ack(msg.entry_id)
        finally:
            structlog.contextvars.unbind_contextvars("job_id", "request_id")

    def _process(self, msg: Message, job: ClaimedJob) -> None:
        node = self._cfg.node_id
        ops = job.pipeline.get("operations", []) if isinstance(job.pipeline, dict) else []
        started = time.monotonic()
        metrics.WORKER_BUSY.labels(node=node).set(1)
        try:
            with (
                heartbeat(self._queue, msg.entry_id, self._log),
                time_limit(self._cfg.job_timeout_seconds),
            ):
                result_key = self._render(job)
        except PermanentError as exc:
            self._db.fail(job.id, node, str(exc))
            metrics.JOBS_PROCESSED.labels(node=node, status="failed").inc()
            self._log.warning("job failed permanently", error=str(exc))
            return
        except Exception as exc:
            error = f"{type(exc).__name__}: {exc}"
            if job.attempts < self._cfg.job_max_attempts:
                if self._db.retry(job.id, node, error):
                    self._queue.requeue(job.id, msg.request_id)
                metrics.JOBS_PROCESSED.labels(node=node, status="retry").inc()
                self._log.warning("job failed, will retry", error=error, attempts=job.attempts)
            else:
                self._db.fail(job.id, node, error)
                metrics.JOBS_PROCESSED.labels(node=node, status="failed").inc()
                self._log.error("job failed, no attempts left", error=error)
            return
        finally:
            metrics.WORKER_BUSY.labels(node=node).set(0)

        elapsed = time.monotonic() - started
        metrics.JOB_PROCESSING_SECONDS.labels(node=node, ops=metrics.ops_bucket(len(ops))).observe(
            elapsed
        )
        if self._db.complete(job.id, node, result_key):
            metrics.JOBS_PROCESSED.labels(node=node, status="done").inc()
            self._log.info("job done", seconds=round(elapsed, 3), result_key=result_key)
        else:
            # another worker took over; it writes the same key, so nothing is lost
            self._log.warning("job was taken over by another worker, result discarded")

    def _render(self, job: ClaimedJob) -> str:
        data = self._storage.get(job.storage_key)
        try:
            img = Image.open(io.BytesIO(data))
            img.load()
        except (
            UnidentifiedImageError,
            Image.DecompressionBombError,
            Image.DecompressionBombWarning,
            OSError,
        ) as exc:
            raise PermanentError(f"cannot decode image: {exc}") from exc
        try:
            out = pipeline.run(img, job.pipeline)
            body, mime, ext = pipeline.encode(out, job.output_format, job.output_quality)
        except pipeline.PipelineError as exc:
            raise PermanentError(f"invalid pipeline: {exc}") from exc
        # deterministic key: re-processing the same job overwrites the same object
        key = f"results/{job.user_id}/{job.id}.{ext}"
        self._storage.put(key, body, mime)
        return key
