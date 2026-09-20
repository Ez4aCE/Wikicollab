-- WikiCollab database schema
-- Migration 001: initial tables

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users
CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(50)  NOT NULL UNIQUE,
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT         NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email    ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- Wikis
CREATE TABLE IF NOT EXISTS wikis (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(255) NOT NULL,
    owner_id   UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_wikis_owner_id ON wikis(owner_id);

-- Pages
CREATE TABLE IF NOT EXISTS pages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wiki_id    UUID         NOT NULL REFERENCES wikis(id) ON DELETE CASCADE,
    title      VARCHAR(255) NOT NULL,
    content    TEXT         NOT NULL DEFAULT '',
    version    INTEGER      NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pages_wiki_id ON pages(wiki_id);

-- Revisions
CREATE TABLE IF NOT EXISTS revisions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    page_id    UUID        NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    content    TEXT        NOT NULL,
    version    INTEGER     NOT NULL,
    created_by UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_revisions_page_id ON revisions(page_id);
CREATE INDEX IF NOT EXISTS idx_revisions_page_version ON revisions(page_id, version);
