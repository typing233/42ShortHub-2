-- 002_analytics_multitenancy.down.sql

DROP MATERIALIZED VIEW IF EXISTS mv_daily_clicks;

DROP INDEX IF EXISTS idx_access_logs_link_time;
DROP INDEX IF EXISTS idx_access_logs_link_referer;
DROP INDEX IF EXISTS idx_access_logs_link_device;
DROP INDEX IF EXISTS idx_access_logs_link_country;
DROP INDEX IF EXISTS idx_access_logs_link_bot;
DROP INDEX IF EXISTS idx_access_logs_link_ip_time;
DROP INDEX IF EXISTS idx_access_logs_time;
DROP INDEX IF EXISTS idx_short_links_batch_job;

DROP TABLE IF EXISTS batch_jobs;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS api_keys;

ALTER TABLE access_logs DROP COLUMN IF EXISTS country;
ALTER TABLE access_logs DROP COLUMN IF EXISTS city;
ALTER TABLE access_logs DROP COLUMN IF EXISTS device_type;
ALTER TABLE access_logs DROP COLUMN IF EXISTS browser;
ALTER TABLE access_logs DROP COLUMN IF EXISTS os;
ALTER TABLE access_logs DROP COLUMN IF EXISTS is_bot;
ALTER TABLE access_logs DROP COLUMN IF EXISTS is_unique;

ALTER TABLE short_links DROP COLUMN IF EXISTS batch_job_id;
