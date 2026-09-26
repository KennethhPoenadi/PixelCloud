"""Retention: delete images (and their results) past `expires_at`.

Runs on every worker, but a Redis lock makes sure only one of them does the
work in a given interval.
"""

from __future__ import annotations

import redis
import structlog

from worker.db import Database
from worker.storage import Storage

LOCK_KEY = "pixelcloud:lock:cleanup"
BATCH_SIZE = 200


def run_cleanup(
    rdb: redis.Redis,
    db: Database,
    storage: Storage,
    node_id: str,
    interval_seconds: int,
    log: structlog.stdlib.BoundLogger,
) -> int:
    """Delete one batch of expired images if we win the lock. Returns images deleted."""
    # The lock lives for the whole interval, so the other workers skip this round.
    if not rdb.set(LOCK_KEY, node_id, nx=True, ex=max(60, interval_seconds)):
        return 0
    expired = db.expired_images(BATCH_SIZE)
    if not expired:
        return 0
    keys = [row["storage_key"] for row in expired]
    for row in expired:
        keys.extend(row["result_keys"] or [])
    # Objects first: if this fails the rows stay and the next round retries.
    storage.remove(keys)
    deleted = db.delete_images([str(row["id"]) for row in expired])
    log.info("retention cleanup", images=deleted, objects=len(keys))
    return deleted
