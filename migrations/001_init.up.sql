-- 001_init.up.sql
-- Initial schema for ShortHub

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL,
    email VARCHAR(128) NOT NULL,
    password VARCHAR(256) NOT NULL,
    role VARCHAR(16) NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_users_username UNIQUE (username),
    CONSTRAINT uk_users_email UNIQUE (email)
);

CREATE TABLE IF NOT EXISTS short_links (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    short_code VARCHAR(32) NOT NULL,
    original_url TEXT NOT NULL,
    title VARCHAR(256) DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    expires_at TIMESTAMPTZ,
    click_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_short_links_code UNIQUE (short_code)
);

CREATE INDEX idx_short_links_user_id ON short_links(user_id);
CREATE INDEX idx_short_links_status ON short_links(status);
CREATE INDEX idx_short_links_expires_at ON short_links(expires_at);

CREATE TABLE IF NOT EXISTS access_logs (
    id BIGSERIAL PRIMARY KEY,
    short_link_id BIGINT NOT NULL,
    ip VARCHAR(45),
    user_agent TEXT,
    referer TEXT,
    accessed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_access_logs_link_id ON access_logs(short_link_id);
CREATE INDEX idx_access_logs_accessed_at ON access_logs(accessed_at);
