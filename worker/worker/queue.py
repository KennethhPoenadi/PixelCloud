"""Reliable job consumption from a Redis Stream consumer group.

- XREADGROUP gives each entry to exactly one worker; it stays in the group's
  pending list until acknowledged.
- While processing, the worker periodically re-claims its own entry (JUSTID),
  which resets the idle timer: that is the heartbeat.
- A worker that dies stops heartbeating, so after `reclaim_idle` another worker
  takes the entry over with XAUTOCLAIM. This is the HA failover path.
- Finished entries are acknowledged and deleted, so XLEN = waiting + in-flight.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

import redis

STREAM = "pixelcloud:jobs"
GROUP = "workers"


@dataclass(frozen=True)
class Message:
    entry_id: str
    job_id: str
    request_id: str
    reclaimed: bool = False


def _decode(entry_id: Any, fields: Any, reclaimed: bool = False) -> Message:
    def text(v: Any) -> str:
        return v.decode() if isinstance(v, bytes) else str(v)

    data = {text(k): text(v) for k, v in (fields or {}).items()}
    return Message(
        entry_id=text(entry_id),
        job_id=data.get("job_id", ""),
        request_id=data.get("request_id", ""),
        reclaimed=reclaimed,
    )


class JobQueue:
    def __init__(self, rdb: redis.Redis, consumer: str) -> None:
        self._rdb = rdb
        self._consumer = consumer
        self._autoclaim_cursor = "0-0"

    def ensure_group(self) -> None:
        try:
            self._rdb.xgroup_create(STREAM, GROUP, id="0", mkstream=True)
        except redis.ResponseError as exc:
            if "BUSYGROUP" not in str(exc):
                raise

    def read(self, block_ms: int) -> Message | None:
        """Wait up to block_ms for a new entry assigned to this consumer."""
        resp = self._rdb.xreadgroup(GROUP, self._consumer, {STREAM: ">"}, count=1, block=block_ms)
        if not resp:
            return None
        _stream, entries = resp[0]
        entry_id, fields = entries[0]
        return _decode(entry_id, fields)

    def reclaim(self, min_idle_ms: int) -> Message | None:
        """Take over one entry whose worker has not heartbeated for min_idle_ms."""
        cursor, claimed, deleted = self._rdb.xautoclaim(
            STREAM,
            GROUP,
            self._consumer,
            min_idle_time=min_idle_ms,
            start_id=self._autoclaim_cursor,
            count=1,
        )
        self._autoclaim_cursor = cursor.decode() if isinstance(cursor, bytes) else str(cursor)
        if deleted:
            # pending entries whose stream data is gone: nothing to process
            self._rdb.xack(STREAM, GROUP, *deleted)
        for entry_id, fields in claimed:
            if fields:
                return _decode(entry_id, fields, reclaimed=True)
            self.ack(entry_id)
        return None

    def heartbeat(self, entry_id: str) -> None:
        """Reset the idle time of an entry we are still working on."""
        self._rdb.xclaim(
            STREAM, GROUP, self._consumer, min_idle_time=0, message_ids=[entry_id], justid=True
        )

    def ack(self, entry_id: str | bytes) -> None:
        pipe = self._rdb.pipeline(transaction=True)
        pipe.xack(STREAM, GROUP, entry_id)
        pipe.xdel(STREAM, entry_id)
        pipe.execute()

    def requeue(self, job_id: str, request_id: str) -> None:
        self._rdb.xadd(
            STREAM,
            {
                "job_id": job_id,
                "request_id": request_id,
                "enqueued_at": datetime.now(UTC).isoformat(),
            },
        )
