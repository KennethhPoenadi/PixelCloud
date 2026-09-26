"""Structured JSON logging to stdout."""

from __future__ import annotations

import logging
import sys

import structlog


def configure(node_id: str) -> structlog.stdlib.BoundLogger:
    logging.basicConfig(format="%(message)s", stream=sys.stdout, level=logging.INFO)
    structlog.configure(
        processors=[
            structlog.contextvars.merge_contextvars,
            structlog.processors.add_log_level,
            structlog.processors.TimeStamper(fmt="iso", utc=True),
            structlog.processors.format_exc_info,
            structlog.processors.JSONRenderer(),
        ],
        logger_factory=structlog.PrintLoggerFactory(file=sys.stdout),
        cache_logger_on_first_use=True,
    )
    logger: structlog.stdlib.BoundLogger = structlog.get_logger().bind(
        service="worker", node_id=node_id
    )
    return logger
