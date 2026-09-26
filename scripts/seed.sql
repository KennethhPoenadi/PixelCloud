-- Idempotent seed: safe to run on every `docker compose up`.
-- Plan numbers are placeholders until the pricing / TCO numbers are final.

INSERT INTO plans (code, name, monthly_quota, max_file_mb, max_resolution, max_batch_size, watermark, api_access, price_idr) VALUES
    ('free',     'Free',     50,   5,  2048, 5,   TRUE,  FALSE, 0),
    ('pro',      'Pro',      2000, 25, 6000, 50,  FALSE, FALSE, 49000),
    ('business', 'Business', NULL, 50, 8000, 200, FALSE, TRUE,  199000)
ON CONFLICT (code) DO UPDATE SET
    name           = EXCLUDED.name,
    monthly_quota  = EXCLUDED.monthly_quota,
    max_file_mb    = EXCLUDED.max_file_mb,
    max_resolution = EXCLUDED.max_resolution,
    max_batch_size = EXCLUDED.max_batch_size,
    watermark      = EXCLUDED.watermark,
    api_access     = EXCLUDED.api_access,
    price_idr      = EXCLUDED.price_idr;

-- System presets: each is a pipeline with a single "preset" op, expanded by the worker.
INSERT INTO filter_presets (user_id, name, pipeline, is_system)
SELECT NULL, p.name, jsonb_build_object('version', 1, 'operations', jsonb_build_array(jsonb_build_object('op', 'preset', 'name', p.name))), TRUE
FROM (VALUES ('grayscale'), ('sepia'), ('vintage'), ('warm'), ('cool'), ('vivid'), ('noir'), ('fade'), ('invert')) AS p(name)
ON CONFLICT (name) WHERE user_id IS NULL DO UPDATE SET
    pipeline  = EXCLUDED.pipeline,
    is_system = TRUE;
