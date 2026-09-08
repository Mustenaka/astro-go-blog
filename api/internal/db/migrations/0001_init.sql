CREATE TABLE assets (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    key           TEXT    NOT NULL UNIQUE,
    prefix        TEXT    NOT NULL,
    original_name TEXT    NOT NULL,
    size          INTEGER NOT NULL,
    mime          TEXT    NOT NULL,
    sha256        TEXT    NOT NULL,
    created_at    TEXT    NOT NULL
);
CREATE INDEX idx_assets_prefix_created ON assets (prefix, created_at DESC);

CREATE TABLE counters (
    slug       TEXT    PRIMARY KEY,
    views      INTEGER NOT NULL DEFAULT 0,
    likes      INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT    NOT NULL
);

CREATE TABLE like_events (
    slug       TEXT NOT NULL,
    ip_hash    TEXT NOT NULL,
    day        TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (slug, ip_hash, day)
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    username   TEXT NOT NULL,
    ip_hash    TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
CREATE INDEX idx_sessions_expires ON sessions (expires_at);

CREATE TABLE build_jobs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    status      TEXT    NOT NULL, -- queued | running | succeeded | failed
    trigger     TEXT    NOT NULL, -- admin | webhook | mcp
    created_at  TEXT    NOT NULL,
    started_at  TEXT,
    finished_at TEXT,
    exit_code   INTEGER,
    log_path    TEXT
);
