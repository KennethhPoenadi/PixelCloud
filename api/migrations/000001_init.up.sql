-- Tiers / pricing plans
CREATE TABLE plans (
    id               SMALLSERIAL PRIMARY KEY,
    code             TEXT UNIQUE NOT NULL,          -- 'free' | 'pro' | 'business'
    name             TEXT NOT NULL,
    monthly_quota    INTEGER,                        -- NULL = unlimited
    max_file_mb      INTEGER NOT NULL,
    max_resolution   INTEGER NOT NULL,              -- px, longest side
    max_batch_size   INTEGER NOT NULL,
    watermark        BOOLEAN NOT NULL DEFAULT FALSE,
    api_access       BOOLEAN NOT NULL DEFAULT FALSE,
    price_idr        INTEGER NOT NULL DEFAULT 0     -- per month
);

CREATE TABLE users (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email          TEXT UNIQUE NOT NULL,
    password_hash  TEXT NOT NULL,
    display_name   TEXT,
    plan_id        SMALLINT NOT NULL REFERENCES plans(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Original uploads
CREATE TABLE images (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_key   TEXT NOT NULL,
    filename      TEXT NOT NULL,
    mime_type     TEXT NOT NULL,
    size_bytes    BIGINT NOT NULL,
    width         INTEGER NOT NULL,
    height        INTEGER NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ
);
CREATE INDEX idx_images_user ON images(user_id, created_at DESC);
CREATE INDEX idx_images_expires ON images(expires_at) WHERE expires_at IS NOT NULL;

-- System presets (user_id NULL) + custom presets owned by a user
CREATE TABLE filter_presets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    pipeline    JSONB NOT NULL,
    is_system   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);
-- UNIQUE (user_id, name) treats NULLs as distinct, so system preset names need their own index
CREATE UNIQUE INDEX uq_system_preset_name ON filter_presets(name) WHERE user_id IS NULL;

-- Batch grouping
CREATE TABLE batches (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pipeline    JSONB NOT NULL,
    total_jobs  INTEGER NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TYPE job_status AS ENUM ('queued', 'processing', 'done', 'failed');

-- One processing job per image
CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    image_id        UUID NOT NULL REFERENCES images(id) ON DELETE CASCADE,
    batch_id        UUID REFERENCES batches(id) ON DELETE SET NULL,
    pipeline        JSONB NOT NULL,
    output_format   TEXT NOT NULL DEFAULT 'jpeg',
    output_quality  SMALLINT DEFAULT 90,
    status          job_status NOT NULL DEFAULT 'queued',
    result_key      TEXT,
    error           TEXT,
    attempts        SMALLINT NOT NULL DEFAULT 0,
    worker_id       TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    finished_at     TIMESTAMPTZ,
    CONSTRAINT jobs_output_format_check CHECK (output_format IN ('jpeg', 'png', 'webp')),
    CONSTRAINT jobs_output_quality_check CHECK (output_quality BETWEEN 1 AND 100)
);
CREATE INDEX idx_jobs_user ON jobs(user_id, created_at DESC);
CREATE INDEX idx_jobs_batch ON jobs(batch_id);
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_image ON jobs(image_id, created_at DESC);

-- Monthly usage counter (quota)
CREATE TABLE usage_monthly (
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    period       DATE NOT NULL,                      -- first day of the month
    jobs_count   INTEGER NOT NULL DEFAULT 0,
    bytes_in     BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, period)
);

-- API keys (Business tier)
CREATE TABLE api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_hash     TEXT NOT NULL UNIQUE,
    label        TEXT,
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_api_keys_user ON api_keys(user_id, created_at DESC);
