"""Prometheus metrics and the small HTTP server exposing /metrics and /healthz."""

from __future__ import annotations

import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from prometheus_client import (
    CONTENT_TYPE_LATEST,
    CollectorRegistry,
    Counter,
    Gauge,
    Histogram,
    gc_collector,
    generate_latest,
    platform_collector,
    process_collector,
)

REGISTRY = CollectorRegistry()
process_collector.ProcessCollector(registry=REGISTRY)
platform_collector.PlatformCollector(registry=REGISTRY)
gc_collector.GCCollector(registry=REGISTRY)

JOBS_PROCESSED = Counter(
    "jobs_processed_total",
    "Jobs finished by this worker, by outcome (done, retry, failed).",
    ["node", "status"],
    registry=REGISTRY,
)
JOB_PROCESSING_SECONDS = Histogram(
    "job_processing_seconds",
    "Time spent processing a job (download, pipeline, upload), by operation-count bucket.",
    ["node", "ops"],
    buckets=(0.05, 0.1, 0.25, 0.5, 1, 2, 4, 8, 15, 30, 60),
    registry=REGISTRY,
)
JOB_QUEUE_WAIT_SECONDS = Histogram(
    "job_queue_wait_seconds",
    "Time between job creation and a worker starting it.",
    ["node"],
    buckets=(0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120),
    registry=REGISTRY,
)
JOBS_RECLAIMED = Counter(
    "jobs_reclaimed_total",
    "Jobs taken over from a worker that stopped heartbeating.",
    ["node"],
    registry=REGISTRY,
)
WORKER_BUSY = Gauge(
    "worker_busy",
    "1 while the worker is processing a job.",
    ["node"],
    registry=REGISTRY,
)


def ops_bucket(count: int) -> str:
    """Bucket the operation count so the label stays low-cardinality."""
    if count <= 1:
        return "0-1"
    if count <= 3:
        return "2-3"
    if count <= 6:
        return "4-6"
    return "7+"


class Heartbeat:
    """Main loop liveness: /healthz fails if the loop stops ticking."""

    def __init__(self, max_age_seconds: float) -> None:
        self._last = time.monotonic()
        self._max_age = max_age_seconds
        self._lock = threading.Lock()

    def beat(self) -> None:
        with self._lock:
            self._last = time.monotonic()

    def healthy(self) -> bool:
        with self._lock:
            return time.monotonic() - self._last < self._max_age


def serve(port: int, heartbeat: Heartbeat) -> ThreadingHTTPServer:
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:
            if self.path == "/metrics":
                body = generate_latest(REGISTRY)
                self._reply(200, body, CONTENT_TYPE_LATEST)
            elif self.path == "/healthz":
                ok = heartbeat.healthy()
                self._reply(200 if ok else 503, b"ok" if ok else b"stale", "text/plain")
            else:
                self._reply(404, b"not found", "text/plain")

        def _reply(self, status: int, body: bytes, content_type: str) -> None:
            self.send_response(status)
            self.send_header("Content-Type", content_type)
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format: str, *args: object) -> None:
            return  # keep stdout for structured logs only

    server = ThreadingHTTPServer(("0.0.0.0", port), Handler)
    threading.Thread(target=server.serve_forever, name="metrics-http", daemon=True).start()
    return server
