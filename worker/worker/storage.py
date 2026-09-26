"""MinIO (S3-compatible) access with bounded timeouts."""

from __future__ import annotations

import io

import urllib3
from minio import Minio
from minio.deleteobjects import DeleteObject


class Storage:
    def __init__(
        self, endpoint: str, access_key: str, secret_key: str, bucket: str, secure: bool
    ) -> None:
        http = urllib3.PoolManager(
            timeout=urllib3.Timeout(connect=5, read=60),
            retries=urllib3.Retry(
                total=3, backoff_factor=0.2, status_forcelist=(500, 502, 503, 504)
            ),
        )
        self._client = Minio(
            endpoint, access_key=access_key, secret_key=secret_key, secure=secure, http_client=http
        )
        self._bucket = bucket

    def get(self, key: str) -> bytes:
        resp = self._client.get_object(self._bucket, key)
        try:
            return resp.read()
        finally:
            resp.close()
            resp.release_conn()

    def put(self, key: str, data: bytes, content_type: str) -> None:
        self._client.put_object(
            self._bucket, key, io.BytesIO(data), length=len(data), content_type=content_type
        )

    def remove(self, keys: list[str]) -> None:
        if not keys:
            return
        errors = list(
            self._client.remove_objects(self._bucket, [DeleteObject(k) for k in keys if k])
        )
        if errors:
            raise RuntimeError(f"failed to delete {len(errors)} object(s): {errors[0]}")
