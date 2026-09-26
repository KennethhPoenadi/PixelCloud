# PixelCloud

Web app filter foto yang jalan sebagai layanan multi-node: upload foto, pilih preset atau atur
adjustment, preview langsung di browser, lalu render resolusi penuh di worker server-side.
Dibangun untuk demo **high availability** (2 API + 2 worker di belakang load balancer) dan
**monitoring nyata** (Prometheus + Grafana), lengkap dengan pricing tier Free / Pro / Business.

```
Browser ─▶ Nginx (LB + frontend) ─▶ api-1 / api-2 (Go) ─▶ Redis Stream ─▶ worker-1 / worker-2 (Python)
                                          │                                   │
                                          └────── Postgres ◀──────────────────┤
                                                  MinIO (S3) ◀────────────────┘
                     Prometheus ◀── /metrics semua node ──▶ Grafana (dashboard ter-provision)
```

## Quick start

Butuh Docker (Compose v2). Semua jalan di container.

```bash
make up          # copy .env.example → .env (kalau belum ada), build, start semua service
make ps          # status + healthcheck
```

| URL | Isi |
|---|---|
| http://localhost:8080 | Aplikasi (landing, gallery, editor) |
| http://localhost:8080/api/v1/healthz | Health — jawaban gantian `api-1` / `api-2` |
| http://localhost:3000 | Grafana (login dari `.env`), dashboard **PixelCloud — Overview** |
| http://localhost:9090 | Prometheus |
| http://localhost:9001 | MinIO console |

Grafana, Prometheus, dan MinIO console cuma di-bind ke `127.0.0.1`; `/metrics` tidak pernah
diproxy Nginx. Kalau port 8080 sudah dipakai, ubah `HTTP_PORT` **dan** `PUBLIC_HOST` di `.env`.

> **Ganti semua nilai `change-me`** di `.env` sebelum dipakai di luar laptop sendiri.

## Fitur

- **Preset** sekali klik: grayscale, sepia, vintage, warm, cool, vivid, noir, fade, invert —
  thumbnail-nya dirender dari foto user sendiri.
- **Adjustment**: brightness, contrast, saturation, temperature, blur, sharpen, vignette;
  **transform**: rotate, flip, resize.
- **Preview** instan pakai CSS/SVG filter. Rumus di worker mengikuti matriks CSS Filter Effects,
  jadi hasil render full-res mirip dengan preview.
- **Custom preset**, **undo/redo**, before/after (tahan `\`), zoom, shortcut `Ctrl/Cmd+Z`,
  `Ctrl/Cmd+Shift+Z`, `E` (export).
- **Export** JPEG / PNG / WebP + quality + batas ukuran.
- **Batch**: satu pipeline untuk banyak foto, progress per foto, unduh ZIP.
- **Kuota per plan** (dicek transaksional), **watermark** untuk Free, **API key** untuk Business.

Angka plan ada di [`scripts/seed.sql`](scripts/seed.sql) (placeholder, gampang diganti).
Untuk pindah plan secara manual:

```bash
docker exec postgres psql -U pixelcloud -d pixelcloud -c \
  "UPDATE users SET plan_id = (SELECT id FROM plans WHERE code = 'pro') WHERE email = 'kamu@example.com'"
```

## Arsitektur

| Service | Peran |
|---|---|
| `nginx` | Load balancer round-robin ke `api-1`/`api-2` (`max_fails`, re-resolve DNS, retry ke node lain), serve frontend, proxy presigned URL MinIO, rate limit per IP |
| `api-1`, `api-2` | Go (chi, pgx, go-redis, minio-go). Stateless. Validasi upload & pipeline, kuota, enqueue job |
| `worker-1`, `worker-2` | Python (Pillow + NumPy). Consumer group Redis Stream, render, upload hasil |
| `postgres` | Data user, plan, image, job, kuota |
| `redis` | Antrian job (Stream + consumer group), rate limit per user, lock cleanup |
| `minio` | Object storage (bucket private, akses via presigned URL 10 menit) |
| `prometheus`, `grafana`, `redis-exporter`, `postgres-exporter` | Monitoring; `cadvisor` opsional (`--profile cadvisor`) |

### Alur job

1. `POST /api/v1/jobs` → dalam **satu transaksi**: cek kepemilikan image, `UPDATE usage_monthly
   … WHERE jobs_count + n <= quota`, insert job `queued`. Lalu `XADD` ke stream.
2. Worker `XREADGROUP` → `UPDATE jobs SET status='processing', worker_id=…`.
3. Selama proses, worker **heartbeat** (`XCLAIM … JUSTID`) tiap 5 detik.
4. Download original → pipeline → watermark (Free) → upload `results/{user}/{job}.{ext}` →
   `status='done'` (update bersyarat `worker_id`), `XACK` + `XDEL`.
5. Gagal → retry maks `JOB_MAX_ATTEMPTS` (3), lalu `failed`.
6. Worker mati → heartbeat berhenti → setelah `RECLAIM_IDLE_SECONDS` worker lain `XAUTOCLAIM`
   job itu. Key hasil deterministik, jadi proses ulang idempotent.

### High availability

- API & worker **stateless** (semua state di Postgres/Redis/MinIO).
- Readiness ≠ liveness: `/readyz` cek Postgres, Redis, MinIO; node yang tidak ready (atau sedang
  shutdown) membalas 503 dan Nginx langsung mencoba node lain.
- Graceful shutdown: API berhenti terima traffic lalu menyelesaikan request; worker
  menyelesaikan job aktif saat SIGTERM (`stop_grace_period: 75s`).
- **SPOF yang diakui:** Postgres dan Redis single instance (MVP). Redis pakai AOF supaya antrian
  selamat dari restart.

### Demo failover

```bash
make failover
```

Script [`scripts/failover-demo.sh`](scripts/failover-demo.sh): jalankan k6 load test, `docker stop
api-1` (request tetap sukses via api-2), tunggu worker-1 memproses job lalu `docker kill worker-1`
(simulasi crash) dan tunjukkan job itu di-reclaim & diselesaikan worker-2, lalu hidupkan lagi.
Buka Grafana selama demo: panel request rate per node, node availability, dan "Jobs reclaimed".

## Monitoring

Semua metric punya label `node`; label tidak pernah berisi user/job ID.

- API: `http_requests_total{node,method,route,status}`, `http_request_duration_seconds`,
  `jobs_enqueued_total`, `quota_rejections_total`, `job_queue_length`, `job_queue_pending`
- Worker: `jobs_processed_total{node,status}`, `job_processing_seconds{node,ops}`,
  `job_queue_wait_seconds`, `jobs_reclaimed_total`, `worker_busy`
- Infra: `redis_*`, `pg_*`, CPU/RSS per proses, CPU/RAM per container (cAdvisor, opsional)

Dashboard di-provision dari [`deploy/grafana/dashboards/pixelcloud.json`](deploy/grafana/dashboards/pixelcloud.json):
availability (bukti SLA), request rate & p95 per node, error rate, queue length, throughput &
processing time per worker, node availability timeline.

Log semua service berupa JSON di stdout dengan `node_id`, `request_id`, `user_id`, `job_id`.
`request_id` dibuat Nginx dan ikut di payload job, jadi satu request bisa ditelusuri dari Nginx →
API → worker.

## API

Base `/api/v1`, spesifikasi lengkap di [`api/openapi.yaml`](api/openapi.yaml). Tipe TypeScript
frontend digenerate dari spec itu (`npm run gen:api`).

```bash
BASE=http://localhost:8080/api/v1
TOKEN=$(curl -s -X POST $BASE/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"kamu@example.com","password":"rahasia123"}' | jq -r .token)

IMG=$(curl -s -X POST $BASE/images -H "Authorization: Bearer $TOKEN" -F file=@foto.jpg | jq -r .id)

JOB=$(curl -s -X POST $BASE/jobs -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{
  "image_id": "'$IMG'",
  "pipeline": {"version": 1, "operations": [
    {"op": "preset", "name": "vintage"},
    {"op": "brightness", "value": 1.1},
    {"op": "resize", "max_width": 1920, "max_height": 1920}
  ]},
  "output_format": "webp", "output_quality": 85
}' | jq -r .id)

curl -s $BASE/jobs/$JOB -H "Authorization: Bearer $TOKEN" | jq '{status, worker_id, download_url}'
```

Plan Business bisa membuat API key di halaman Akun lalu memakai header `X-API-Key: pc_…`
sebagai ganti `Authorization`.

Error selalu `{"error": {"code": "...", "message": "..."}}` dengan kode tetap:
`VALIDATION_ERROR`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, `QUOTA_EXCEEDED`,
`FILE_TOO_LARGE`, `UNSUPPORTED_FORMAT`, `RATE_LIMITED`, `UNAVAILABLE`, `INTERNAL`.

## Development

```bash
make test        # go test + pytest
make lint        # gofmt/vet/golangci-lint + ruff/mypy
make test-e2e    # Playwright (stack harus jalan): register → upload → preset → download
make loadtest    # k6 via Docker, DURATION=3m VUS=10
make logs        # log api & worker
make clean       # stop + hapus semua volume data
```

Frontend dev server (proxy ke stack yang sedang jalan):

```bash
cd frontend && npm install && PIXELCLOUD_URL=http://localhost:8080 npm run dev
```

Worker lokal: `cd worker && uv sync && uv run pytest`. Golden image test di
`worker/tests/golden/`; setelah mengubah filter secara sengaja jalankan
`UPDATE_GOLDEN=1 uv run pytest tests/test_golden.py`.

CI ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)): lint + test tiap service,
`govulncheck`, `pip-audit`, `npm audit`, cek tipe OpenAPI up to date, build image, dan E2E
Playwright terhadap stack Compose.

## Konfigurasi

Semua lewat environment ([`.env.example`](.env.example)). Yang penting:

| Variable | Default | Keterangan |
|---|---|---|
| `HTTP_PORT` / `PUBLIC_HOST` | `8080` / `localhost:8080` | Port publik Nginx; presigned URL ditandatangani untuk host ini |
| `JWT_SECRET`, `JWT_TTL` | – / `1h` | Secret minimal 32 byte |
| `PRESIGN_TTL` | `10m` | Umur URL gambar |
| `RETENTION_DAYS` | `30` | File dihapus otomatis setelah ini |
| `USER_RATE_LIMIT_PER_MIN` | `300` | Limit per user, dibagi semua node via Redis |
| `JOB_TIMEOUT_SECONDS`, `JOB_MAX_ATTEMPTS` | `60`, `3` | Batas waktu & retry per job |
| `RECLAIM_IDLE_SECONDS` | `15` | Job worker yang diam selama ini diambil alih |

## Security baseline

- Password bcrypt cost 12; login tanpa user enumeration (timing sama).
- JWT HS256 1 jam; API key disimpan sebagai SHA-256 hash, plaintext ditampilkan sekali.
- Upload: magic bytes (bukan `Content-Type` client), whitelist JPEG/PNG/WebP, batas ukuran &
  resolusi per plan, **EXIF/GPS/XMP/komentar di-strip tanpa re-encode** (orientasi dipertahankan).
- Worker: `MAX_IMAGE_PIXELS` + decompression bomb jadi error, timeout per job, output tanpa metadata.
- Pipeline divalidasi di API **dan** worker (op tak dikenal ditolak, maks 20 operasi, range per op).
- Bucket private, presigned URL 10 menit.
- Rate limit Nginx per IP (API 50 r/s, auth 10 r/menit) + per user di API.
- Retensi: worker menghapus image & hasil lewat `expires_at` (satu worker per interval via lock Redis).
- Header keamanan (CSP, `X-Frame-Options`, `nosniff`, dll.), container non-root, image slim/distroless.
- Secret hanya lewat `.env` (di-`.gitignore`).

## Catatan

- Image resmi MinIO sudah tidak dipublikasikan; compose memakai `pgsty/minio`, build komunitas dari
  source MinIO upstream. Bisa diganti S3-compatible lain tanpa ubah kode.
- Nginx me-retry request (termasuk POST) ke node lain kalau node pertama gagal sebelum membalas.
  Ini membuat failover mulus, dengan risiko kecil job dobel kalau node crash tepat saat memproses
  `POST /jobs`.

## Struktur repo

```
api/        Go API (cmd/api, internal/{auth,handlers,pipeline,quota,queue,storage,metrics,db,...}), migrations, openapi.yaml
worker/     Python worker (filters/ satu file per operasi + presets.py, pipeline.py, queue.py, processor.py, ...)
frontend/   React + TS + Vite + Tailwind (pages, components, lib), Playwright e2e
deploy/     nginx, prometheus, grafana (provisioning + dashboard JSON)
scripts/    seed.sql, loadtest.js (k6), failover-demo.sh
docs/       implementation plan
```
