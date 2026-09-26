from __future__ import annotations

from typing import Any

import structlog

from worker.cleanup import run_cleanup


class FakeRedis:
    def __init__(self) -> None:
        self.store: dict[str, str] = {}

    def set(self, key: str, value: str, nx: bool, ex: int) -> bool:
        if nx and key in self.store:
            return False
        self.store[key] = value
        return True


class FakeDB:
    def __init__(self, rows: list[dict[str, Any]]) -> None:
        self.rows = rows
        self.deleted: list[str] = []

    def expired_images(self, limit: int) -> list[dict[str, Any]]:
        return self.rows[:limit]

    def delete_images(self, ids: list[str]) -> int:
        self.deleted.extend(ids)
        return len(ids)


class FakeStorage:
    def __init__(self) -> None:
        self.removed: list[str] = []

    def remove(self, keys: list[str]) -> None:
        self.removed.extend(keys)


def test_cleanup_deletes_objects_then_rows_once_per_interval() -> None:
    rdb, storage = FakeRedis(), FakeStorage()
    db = FakeDB(
        [
            {"id": "a", "storage_key": "uploads/u/a.jpg", "result_keys": ["results/u/j1.jpg"]},
            {"id": "b", "storage_key": "uploads/u/b.png", "result_keys": []},
        ]
    )
    log = structlog.get_logger()
    args = (rdb, db, storage, "worker-1", 3600, log)
    assert run_cleanup(*args) == 2  # type: ignore[arg-type]
    assert storage.removed == ["uploads/u/a.jpg", "uploads/u/b.png", "results/u/j1.jpg"]
    assert db.deleted == ["a", "b"]
    # second worker in the same interval does nothing
    assert run_cleanup(rdb, db, storage, "worker-2", 3600, log) == 0  # type: ignore[arg-type]
