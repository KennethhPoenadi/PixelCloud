"""Worker entrypoint: consume jobs until SIGTERM, then exit cleanly.

On SIGTERM the current job is finished before exiting (graceful shutdown), so
`docker stop` never leaves a half-written result. A crash (`docker kill`) is
covered by the reclaim path instead.
"""

from __future__ import annotations

import signal
import threading
import time
from types import FrameType

import redis

from worker import metrics
from worker.config import Config
from worker.db import Database
from worker.log import configure
from worker.processor import Processor
from worker.queue import JobQueue
from worker.storage import Storage

READ_BLOCK_MS = 2000
RECLAIM_EVERY_SECONDS = 5.0


def main() -> None:
    cfg = Config.from_env()
    log = configure(cfg.node_id)

    stop = threading.Event()

    def on_signal(signum: int, _frame: FrameType | None) -> None:
        log.info("shutdown requested, finishing current job", signal=signal.Signals(signum).name)
        stop.set()

    signal.signal(signal.SIGTERM, on_signal)
    signal.signal(signal.SIGINT, on_signal)

    beat = metrics.Heartbeat(max_age_seconds=cfg.job_timeout_seconds + 30)
    metrics.serve(cfg.metrics_port, beat)
    metrics.WORKER_BUSY.labels(node=cfg.node_id).set(0)

    rdb = redis.Redis.from_url(
        cfg.redis_url, socket_timeout=10, socket_connect_timeout=5, health_check_interval=30
    )
    queue = JobQueue(rdb, consumer=cfg.node_id)
    db = Database(cfg.database_url)
    storage = Storage(
        cfg.minio_endpoint,
        cfg.minio_access_key,
        cfg.minio_secret_key,
        cfg.minio_bucket,
        cfg.minio_use_ssl,
    )
    processor = Processor(cfg, db, storage, queue, log)
    log.info("worker started", metrics_port=cfg.metrics_port)

    group_ready = False
    last_reclaim = 0.0
    while not stop.is_set():
        beat.beat()
        try:
            if not group_ready:
                queue.ensure_group()
                group_ready = True
                log.info("consumer group ready")

            msg = None
            if time.monotonic() - last_reclaim >= RECLAIM_EVERY_SECONDS:
                last_reclaim = time.monotonic()
                msg = queue.reclaim(cfg.reclaim_idle_seconds * 1000)
            if msg is None:
                msg = queue.read(READ_BLOCK_MS)
            if msg is not None:
                processor.handle(msg)
        except Exception as exc:
            log.error("worker loop error, backing off", error=f"{type(exc).__name__}: {exc}")
            stop.wait(2)

    log.info("worker stopped")


if __name__ == "__main__":
    main()
