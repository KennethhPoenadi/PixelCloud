"""Worker settings, loaded from environment variables (12-factor)."""

from __future__ import annotations

import os
import socket
from dataclasses import dataclass


def _env(key: str, default: str | None = None) -> str:
    value = os.environ.get(key) or default
    if value is None:
        raise RuntimeError(f"{key} is required")
    return value


def _bool(key: str, default: bool) -> bool:
    return _env(key, str(default)).lower() in {"1", "true", "yes"}


@dataclass(frozen=True)
class Config:
    node_id: str
    database_url: str
    redis_url: str
    minio_endpoint: str
    minio_access_key: str
    minio_secret_key: str
    minio_bucket: str
    minio_use_ssl: bool
    metrics_port: int
    job_timeout_seconds: int
    job_max_attempts: int
    reclaim_idle_seconds: int
    cleanup_interval_seconds: int
    max_image_pixels: int

    @classmethod
    def from_env(cls) -> Config:
        return cls(
            node_id=_env("NODE_ID", socket.gethostname()),
            database_url=_env("DATABASE_URL"),
            redis_url=_env("REDIS_URL"),
            minio_endpoint=_env("MINIO_ENDPOINT"),
            minio_access_key=_env("MINIO_ACCESS_KEY"),
            minio_secret_key=_env("MINIO_SECRET_KEY"),
            minio_bucket=_env("MINIO_BUCKET", "pixelcloud"),
            minio_use_ssl=_bool("MINIO_USE_SSL", False),
            metrics_port=int(_env("METRICS_PORT", "9100")),
            job_timeout_seconds=int(_env("JOB_TIMEOUT_SECONDS", "60")),
            job_max_attempts=int(_env("JOB_MAX_ATTEMPTS", "3")),
            reclaim_idle_seconds=int(_env("RECLAIM_IDLE_SECONDS", "15")),
            cleanup_interval_seconds=int(_env("CLEANUP_INTERVAL_SECONDS", "3600")),
            # decompression-bomb guard; largest plan allows 8000px on the long side
            max_image_pixels=int(_env("MAX_IMAGE_PIXELS", str(8000 * 8000))),
        )
