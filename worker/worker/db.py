"""Job state transitions in Postgres.

Every update is conditional (status + worker_id), so a worker that was presumed
dead and later wakes up cannot overwrite the result of the worker that took
its job over.
"""

from __future__ import annotations

import threading
from dataclasses import dataclass
from datetime import datetime
from typing import Any

import psycopg
from psycopg.rows import dict_row


@dataclass(frozen=True)
class ClaimedJob:
    id: str
    user_id: str
    image_id: str
    pipeline: Any
    output_format: str
    output_quality: int
    attempts: int
    created_at: datetime
    storage_key: str
    watermark: bool


class Database:
    def __init__(self, url: str) -> None:
        self._url = url
        self._conn: psycopg.Connection[dict[str, Any]] | None = None
        self._lock = threading.Lock()

    def _connection(self) -> psycopg.Connection[dict[str, Any]]:
        if self._conn is None or self._conn.closed or self._conn.broken:
            self._conn = psycopg.connect(
                self._url,
                autocommit=True,
                connect_timeout=5,
                options="-c statement_timeout=5000",
                row_factory=dict_row,
            )
        return self._conn

    def _one(self, sql: str, params: dict[str, Any]) -> dict[str, Any] | None:
        with self._lock:
            try:
                return self._connection().execute(sql, params).fetchone()
            except psycopg.OperationalError:
                self._conn = None  # reconnect next time
                raise

    def _all(self, sql: str, params: dict[str, Any]) -> list[dict[str, Any]]:
        with self._lock:
            try:
                return self._connection().execute(sql, params).fetchall()
            except psycopg.OperationalError:
                self._conn = None
                raise

    def ping(self) -> None:
        self._one("SELECT 1", {})

    def claim(self, job_id: str, worker_id: str) -> ClaimedJob | None:
        """Mark a queued (or abandoned) job as processing by this worker."""
        row = self._one(
            """
            WITH j AS (
                UPDATE jobs
                SET status = 'processing', worker_id = %(worker)s, started_at = now(),
                    attempts = attempts + 1, error = NULL
                WHERE id = %(id)s AND status IN ('queued', 'processing')
                RETURNING id, user_id, image_id, pipeline, output_format, output_quality,
                          attempts, created_at
            )
            SELECT j.*, i.storage_key, p.watermark
            FROM j
            JOIN images i ON i.id = j.image_id
            JOIN users u ON u.id = j.user_id
            JOIN plans p ON p.id = u.plan_id
            """,
            {"id": job_id, "worker": worker_id},
        )
        if row is None:
            return None
        return ClaimedJob(
            id=str(row["id"]),
            user_id=str(row["user_id"]),
            image_id=str(row["image_id"]),
            pipeline=row["pipeline"],
            output_format=row["output_format"],
            output_quality=int(row["output_quality"] or 90),
            attempts=int(row["attempts"]),
            created_at=row["created_at"],
            storage_key=row["storage_key"],
            watermark=bool(row["watermark"]),
        )

    def complete(self, job_id: str, worker_id: str, result_key: str) -> bool:
        row = self._one(
            """
            UPDATE jobs SET status = 'done', result_key = %(key)s, finished_at = now(), error = NULL
            WHERE id = %(id)s AND status = 'processing' AND worker_id = %(worker)s
            RETURNING id
            """,
            {"id": job_id, "worker": worker_id, "key": result_key},
        )
        return row is not None

    def retry(self, job_id: str, worker_id: str, error: str) -> bool:
        row = self._one(
            """
            UPDATE jobs SET status = 'queued', error = %(error)s
            WHERE id = %(id)s AND status = 'processing' AND worker_id = %(worker)s
            RETURNING id
            """,
            {"id": job_id, "worker": worker_id, "error": error[:500]},
        )
        return row is not None

    def fail(self, job_id: str, worker_id: str, error: str) -> bool:
        row = self._one(
            """
            UPDATE jobs SET status = 'failed', error = %(error)s, finished_at = now()
            WHERE id = %(id)s AND status = 'processing' AND worker_id = %(worker)s
            RETURNING id
            """,
            {"id": job_id, "worker": worker_id, "error": error[:500]},
        )
        return row is not None

    def expired_images(self, limit: int) -> list[dict[str, Any]]:
        """Images past their retention date, with every object key they own."""
        return self._all(
            """
            SELECT i.id, i.storage_key,
                   coalesce(array_agg(j.result_key) FILTER (WHERE j.result_key IS NOT NULL), '{}')
                       AS result_keys
            FROM images i LEFT JOIN jobs j ON j.image_id = i.id
            WHERE i.expires_at IS NOT NULL AND i.expires_at < now()
            GROUP BY i.id
            ORDER BY i.expires_at
            LIMIT %(limit)s
            """,
            {"limit": limit},
        )

    def delete_images(self, ids: list[str]) -> int:
        rows = self._all(
            "DELETE FROM images WHERE id = ANY(%(ids)s::uuid[]) RETURNING id", {"ids": ids}
        )
        return len(rows)
