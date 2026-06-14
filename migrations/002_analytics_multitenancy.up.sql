-- 002_analytics_multitenancy.up.sql
-- Adds analytics enrichment columns, API keys, audit logs, batch jobs, and performance indexes.

-- Extend access_logs with analytics columns
ALTER TABLE access_logs ADD COLUMN IF NOT EXISTS country VARCHAR(2) DEFAULT '';
ALTER TABLE access_logs ADD COLUMN IF NOT EXISTS city VARCHAR(128) DEFAULT '';
ALTER TABLE access_logs ADD COLUMN IF NOT EXISTS device_type VARCHAR(16) DEFAULT '';
ALTER TABLE access_logs ADD COLUMN IF NOT EXISTS browser VARCHAR(64) DEFAULT '';
ALTER TABLE access_logs ADD COLUMN IF NOT EXISTS os VARCHAR(64) DEFAULT '';
ALTER TABLE access_logs ADD COLUMN IF NOT EXISTS is_bot BOOLEAN DEFAULT FALSE;
ALTER TABLE access_logs ADD COLUMN IF NOT EXISTS is_unique BOOLEAN DEFAULT TRUE;

-- Extend short_links with batch job reference
ALTER TABLE short_links ADD COLUMN IF NOT EXISTS batch_job_id BIGINT;

-- API Keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    name VARCHAR(128) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    prefix VARCHAR(8) NOT NULL,
    quota_daily BIGINT NOT NULL DEFAULT 1000,
    rate_per_min INT NOT NULL DEFAULT 60,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(prefix);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);

-- Audit Logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT,
    action VARCHAR(64) NOT NULL,
    resource VARCHAR(32),
    resource_id BIGINT,
    detail TEXT,
    ip VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);

-- Batch Jobs table
CREATE TABLE IF NOT EXISTS batch_jobs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    type VARCHAR(32) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    total_items INT NOT NULL,
    processed_items INT NOT NULL DEFAULT 0,
    success_count INT NOT NULL DEFAULT 0,
    fail_count INT NOT NULL DEFAULT 0,
    result_json TEXT,
    error_json TEXT,
    idempotency_key VARCHAR(64) UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_batch_jobs_user_id ON batch_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_batch_jobs_status ON batch_jobs(status);
CREATE INDEX IF NOT EXISTS idx_batch_jobs_idempotency ON batch_jobs(idempotency_key);

-- Performance indexes for analytics queries
CREATE INDEX IF NOT EXISTS idx_access_logs_link_time ON access_logs(short_link_id, accessed_at DESC);
CREATE INDEX IF NOT EXISTS idx_access_logs_link_referer ON access_logs(short_link_id, referer) WHERE referer != '';
CREATE INDEX IF NOT EXISTS idx_access_logs_link_device ON access_logs(short_link_id, device_type);
CREATE INDEX IF NOT EXISTS idx_access_logs_link_country ON access_logs(short_link_id, country);
CREATE INDEX IF NOT EXISTS idx_access_logs_link_bot ON access_logs(short_link_id, is_bot);
CREATE INDEX IF NOT EXISTS idx_access_logs_link_ip_time ON access_logs(short_link_id, ip, accessed_at DESC);
CREATE INDEX IF NOT EXISTS idx_access_logs_time ON access_logs(accessed_at DESC);
CREATE INDEX IF NOT EXISTS idx_short_links_batch_job ON short_links(batch_job_id) WHERE batch_job_id IS NOT NULL;

-- Materialized view for daily aggregated click stats
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_daily_clicks AS
SELECT
    short_link_id,
    DATE(accessed_at) AS click_date,
    COUNT(*) AS total_clicks,
    COUNT(*) FILTER (WHERE is_unique) AS unique_clicks,
    COUNT(*) FILTER (WHERE is_bot = FALSE) AS human_clicks
FROM access_logs
GROUP BY short_link_id, DATE(accessed_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mv_daily_clicks_pk ON mv_daily_clicks(short_link_id, click_date);
