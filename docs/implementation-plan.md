# PixelCloud — Implementation Plan

> Scope: **web app filter foto** (open-source style, simple, fokus ke filter), jalan sebagai layanan multi-node dengan HA + monitoring.
> Deployment target: **lokal via Docker Compose** (budget constraint), arsitektur tetap cloud-ready.

---

## 1. Goals

- User bisa upload foto, apply filter (preset + adjustment manual), preview, lalu download hasilnya.
- Processing dilakukan di server (worker), bukan cuma di browser → ada beban nyata untuk dimonitor.
- Minimal **2 node API + 2 node worker** di belakang load balancer (HA, bisa demo failover).
- **Real metrics** (Prometheus + Grafana).
- Mendukung **pricing tier** (Free / Pro / Business) lewat kuota di database.

### Out of scope (MVP)
- AI features (background removal, upscaling)
- Kolaborasi / sharing antar user
- Mobile app
- Payment gateway beneran (tier di-set manual / via seed)

---

## 2. Features

### 2.1 Filter (core)

**Preset filters** (satu klik):
| Preset | Deskripsi singkat |
|---|---|
| `grayscale` | Hitam putih |
| `sepia` | Nada coklat klasik |
| `vintage` | Sepia ringan + contrast turun + vignette |
| `warm` | Geser white balance ke oranye |
| `cool` | Geser white balance ke biru |
| `vivid` | Saturation + contrast naik |
| `noir` | Grayscale high-contrast |
| `fade` | Blacks diangkat, saturation turun |
| `invert` | Negatif |

**Adjustments** (slider, bisa dikombinasi):
| Operation | Range | Default |
|---|---|---|
| `brightness` | 0.0 – 2.0 | 1.0 |
| `contrast` | 0.0 – 2.0 | 1.0 |
| `saturation` | 0.0 – 2.0 | 1.0 |
| `temperature` | -100 – 100 | 0 |
| `blur` | 0 – 20 (radius px) | 0 |
| `sharpen` | 0.0 – 3.0 | 0 |
| `vignette` | 0.0 – 1.0 | 0 |

**Transform dasar**: `rotate` (90/180/270), `flip` (h/v), `resize` (max width/height, keep ratio).

### 2.2 Pipeline
- Filter disimpan sebagai **pipeline JSON** (urutan operasi) → bisa di-reuse & disimpan sebagai **custom preset** milik user.
- Preview cepat di browser pakai CSS filter / canvas pada gambar low-res; **render final** di worker (full resolution).

### 2.3 Batch
- Upload banyak foto sekaligus, apply 1 pipeline ke semuanya → 1 job per gambar, download hasil sebagai ZIP.

### 2.4 Lainnya
- Auth (register/login, JWT)
- History: daftar foto & hasil edit per user
- Output format: JPEG / PNG / WebP (+ quality untuk JPEG/WebP)
- Kuota per tier (lihat §5)
- Watermark otomatis untuk tier Free

---

## 3. Architecture

```
                 ┌──────────────┐
  Browser ─────▶ │    Nginx     │  (load balancer + serve frontend)
                 └──────┬───────┘
              round-robin│
             ┌──────────┴──────────┐
        ┌────▼────┐           ┌────▼────┐
        │ api-1   │           │ api-2   │   (stateless, Go)
        └────┬────┘           └────┬────┘
             │   enqueue job        │
             └────────┬─────────────┘
                 ┌────▼────┐
                 │  Redis  │  (job queue)
                 └────┬────┘
             ┌────────┴──────────┐
        ┌────▼─────┐        ┌────▼─────┐
        │ worker-1 │        │ worker-2 │  (stateless, Python)
        └────┬─────┘        └────┬─────┘
             └───────┬───────────┘
        ┌────────────┼──────────────┐
   ┌────▼─────┐ ┌────▼────┐   ┌─────▼──────┐
   │ Postgres │ │  MinIO  │   │ Prometheus │──▶ Grafana
   └──────────┘ │ (S3)    │   └────────────┘
                └─────────┘
```

- API dan worker **stateless** → bisa di-scale / dimatikan satu tanpa downtime.
- File disimpan di **MinIO** (S3-compatible), bukan di disk container.
- Postgres & Redis single instance di MVP (diakui sebagai SPOF di dokumen; opsional replica kalau ada waktu).

---

## 4. Tech Stack

| Layer | Pilihan |
|---|---|
| Frontend | React + TypeScript + Vite, Tailwind |
| API | Go (chi atau gin), `pgx`, `go-redis`, `minio-go`, `prometheus/client_golang` |
| Worker | Python 3.12, Pillow (+ NumPy untuk vignette/temperature), `redis`, `boto3`/`minio`, `prometheus_client` |
| Queue | Redis (list `BRPOPLPUSH` / stream + consumer group) |
| DB | PostgreSQL 16 |
| Object storage | MinIO |
| LB | Nginx |
| Monitoring | Prometheus + Grafana (+ node/cAdvisor exporter opsional) |
| Orchestration | Docker Compose |

---

## 5. Database Schema

Lihat `api/migrations/000001_init.up.sql`. Tabel: `plans`, `users`, `images`, `filter_presets`, `batches`, `jobs`, `usage_monthly`, `api_keys`.

**Seed data**: 3 baris `plans` (angka final nunggu hitungan pricing/TCO), dan preset sistem dari §2.1 ke `filter_presets` dengan `is_system = TRUE`.

Contoh seed awal (placeholder, nanti diganti angka dari tim bisnis):
| code | monthly_quota | max_file_mb | max_resolution | max_batch_size | watermark | api_access |
|---|---|---|---|---|---|---|
| free | 50 | 5 | 2048 | 5 | true | false |
| pro | 2000 | 25 | 6000 | 50 | false | false |
| business | NULL | 50 | 8000 | 200 | false | true |

---

## 6. Pipeline JSON Format

```json
{
  "version": 1,
  "operations": [
    { "op": "preset", "name": "vintage" },
    { "op": "brightness", "value": 1.1 },
    { "op": "contrast", "value": 1.2 },
    { "op": "vignette", "value": 0.4 },
    { "op": "resize", "max_width": 1920, "max_height": 1920 }
  ]
}
```

Aturan:
- Operasi dieksekusi berurutan.
- API **memvalidasi** setiap `op` & range value sebelum enqueue (tolak op tak dikenal, max 20 operasi).
- `preset` di-expand oleh worker ke operasi dasarnya (definisi preset ada di satu modul worker).
- Worker juga validasi ulang (defense in depth).

---

## 7. API Endpoints

Base: `/api/v1`. Auth: `Authorization: Bearer <jwt>` atau `X-API-Key` (Business). Spesifikasi lengkap: `api/openapi.yaml`.

| Method | Path | Deskripsi |
|---|---|---|
| POST | `/auth/register` | Register |
| POST | `/auth/login` | Login → JWT |
| GET | `/me` | Profil + plan + usage bulan ini |
| POST | `/images` | Upload (multipart), validasi size/mime/resolusi sesuai plan |
| GET | `/images` | List foto user (paginated) |
| GET | `/images/{id}` | Detail + presigned URL original |
| DELETE | `/images/{id}` | Hapus foto + hasil |
| GET | `/presets` | Preset sistem + custom user |
| POST | `/presets` | Simpan custom preset |
| DELETE | `/presets/{id}` | Hapus custom preset |
| POST | `/jobs` | Body: `image_id`, `pipeline`, `output_format`, `output_quality` → cek kuota, enqueue |
| GET | `/jobs/{id}` | Status + presigned URL hasil kalau `done` |
| POST | `/batches` | Body: `image_ids[]`, `pipeline`, format → enqueue N job |
| GET | `/batches/{id}` | Progress (done/total) |
| GET | `/batches/{id}/download` | ZIP hasil |
| GET | `/healthz` | Liveness |
| GET | `/readyz` | Readiness (cek DB, Redis, MinIO) |
| GET | `/metrics` | Prometheus (tidak diekspos lewat Nginx publik) |

Response error konsisten: `{ "error": { "code": "QUOTA_EXCEEDED", "message": "..." } }`.

---

## 8. Job Flow

1. `POST /jobs` → API cek kuota (`usage_monthly` vs `plans.monthly_quota`) dalam 1 transaksi, insert `jobs` (status `queued`), increment usage, push `job_id` ke Redis.
2. Worker ambil job secara **reliable** (Redis Streams consumer group, atau `BRPOPLPUSH` ke list processing) → set `processing`, `started_at`, `worker_id`.
3. Worker download original dari MinIO → jalankan pipeline → watermark kalau plan butuh → upload hasil ke `results/{user_id}/{job_id}.{ext}` → set `done`, `result_key`, `finished_at`.
4. Gagal → `attempts++`, retry maks 3 kali, lalu `failed` + `error`.
5. Job yang stuck di `processing` > N detik (worker mati) di-reclaim oleh worker lain → **inilah yang didemokan untuk HA**.
6. Frontend polling `GET /jobs/{id}` tiap 1 detik (MVP; SSE opsional).

---

## 9. HA Setup (Docker Compose)

Services: `nginx`, `api-1`, `api-2`, `worker-1`, `worker-2`, `postgres`, `redis`, `minio`, `prometheus`, `grafana`, `frontend` (build static, di-serve nginx).

- Nginx upstream `api` → `api-1`, `api-2` dengan `max_fails` / `fail_timeout` supaya node mati otomatis di-skip.
- Healthcheck di setiap service (`/healthz`).
- `restart: unless-stopped`.
- Env var `NODE_ID` di tiap api/worker → dimasukkan ke label metrics & kolom `jobs.worker_id`.

**Skenario demo failover** (`scripts/failover-demo.sh`):
1. Jalankan load test kecil (k6 atau script Python) terus-menerus.
2. `docker stop api-1` → request tetap sukses via `api-2`.
3. `docker stop worker-1` di tengah proses → job di-reclaim `worker-2`.
4. Tunjukkan di Grafana: traffic pindah node, error rate tetap rendah.

---

## 10. Monitoring (Real Metrics)

**API (Go)**
- `http_requests_total{node, method, route, status}`
- `http_request_duration_seconds` (histogram)
- `jobs_enqueued_total{node}`
- `quota_rejections_total`

**Worker (Python)**
- `jobs_processed_total{node, status}`
- `job_processing_seconds` (histogram, per op count)
- `job_queue_wait_seconds` (created → started)

**Infra**
- Queue length (redis_exporter atau gauge dari API)
- CPU / RAM per container (cAdvisor)
- Postgres up (postgres_exporter, opsional)

**Grafana dashboard** (provisioned dari file, bukan manual):
- Request rate & p95 latency per node
- Error rate
- Queue length
- Job throughput & processing time per worker
- Uptime / availability (untuk bukti SLA)

---

## 11. Security Baseline

- Password hash: argon2id atau bcrypt (cost ≥ 12).
- JWT expiry pendek (mis. 1 jam), secret dari env.
- API key disimpan sebagai hash.
- Upload: whitelist mime (`image/jpeg`, `image/png`, `image/webp`), cek magic bytes, limit size & resolusi per plan, strip EXIF (privacy — lokasi GPS).
- Pipeline divalidasi di API & worker.
- Presigned URL MinIO dengan expiry pendek (mis. 10 menit); bucket tidak public.
- Rate limiting di Nginx (per IP) + per user di API.
- Retensi: file dihapus otomatis setelah X hari (`expires_at`, cron job cleanup).
- `/metrics` & Grafana tidak diekspos publik.
- Semua secret lewat `.env` (ada `.env.example`, `.env` di `.gitignore`).

---

## 12. Milestones

- **M1 — Skeleton & infra**: compose + healthcheck, migrasi + seed, `/api/v1/healthz` via Nginx balas dari api-1 & api-2 bergantian.
- **M2 — Upload & auth**: register/login, upload ke MinIO, gallery, validasi plan.
- **M3 — Filter engine**: semua operasi & preset + unit test, job flow lengkap.
- **M4 — Editor UI**: preset, slider, preview, render final, custom preset.
- **M5 — Batch, kuota, watermark**.
- **M6 — Monitoring & HA demo**: metrics, dashboard, reclaim, failover script, load test.
- **M7 — Hardening**: rate limiting, EXIF strip, retensi & cleanup, README.

---

## 13. Engineering Best Practices & UI Design System

Ringkasan aturan yang dipakai di kode:

- Conventional commits, `Makefile` sebagai satu pintu, CI lint + test + build image.
- Go: `gofmt`, `golangci-lint`, error di-wrap dengan konteks, tidak ada `panic` di handler.
- Python: `ruff`, type hints, `mypy` untuk `filters/` dan `pipeline.py`.
- Frontend: TypeScript `strict`, ESLint + Prettier, tanpa `any`.
- 12-factor, timeout di semua I/O, graceful shutdown, worker idempotent, transaksi untuk kuota, readiness ≠ liveness.
- API: `/api/v1`, snake_case, cursor pagination, OpenAPI di `api/openapi.yaml`, kode error tetap.
- Observability: log JSON terstruktur dengan `node_id`, `request_id`, `user_id`, `job_id`; label metrics low-cardinality.
- Testing: unit + golden image (worker), validator, integration (API), E2E Playwright, load test k6.
- UI: tema "Pixel RGB" — base gelap netral, biru `#3D7BFF` satu-satunya primary, merah/hijau hanya status; Space Grotesk / Inter / JetBrains Mono; shadcn/ui (Radix) + lucide-react.
