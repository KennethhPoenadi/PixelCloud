"""Worker entrypoint: consume jobs until SIGTERM, then exit cleanly."""

from __future__ import annotations

import signal
import threading
import time
from types import FrameType

from worker import metrics
from worker.config import Config
from worker.log import configure


def main() -> None:
    cfg = Config.from_env()
    log = configure(cfg.node_id)

    stop = threading.Event()

    def on_signal(signum: int, _frame: FrameType | None) -> None:
        log.info("shutdown requested", signal=signal.Signals(signum).name)
        stop.set()

    signal.signal(signal.SIGTERM, on_signal)
    signal.signal(signal.SIGINT, on_signal)

    heartbeat = metrics.Heartbeat(max_age_seconds=cfg.job_timeout_seconds + 30)
    metrics.serve(cfg.metrics_port, heartbeat)
    log.info("worker started", metrics_port=cfg.metrics_port)

    while not stop.is_set():
        heartbeat.beat()
        time.sleep(1)

    log.info("worker stopped")


if __name__ == "__main__":
    main()
