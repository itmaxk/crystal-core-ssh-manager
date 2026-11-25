CREATE TABLE IF NOT EXISTS ssh_keys (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    key_type        TEXT NOT NULL,
    fingerprint     TEXT NOT NULL,
    public_key      TEXT NOT NULL,
    private_key_enc BLOB NOT NULL,
    passphrase_enc  BLOB,
    is_default      INTEGER NOT NULL DEFAULT 0,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS hosts (
    id                TEXT PRIMARY KEY,
    name              TEXT NOT NULL,
    hostname          TEXT NOT NULL,
    port              INTEGER NOT NULL DEFAULT 22,
    username          TEXT NOT NULL DEFAULT 'root',
    key_id            TEXT REFERENCES ssh_keys(id) ON DELETE SET NULL,
    jump_host_id      TEXT REFERENCES hosts(id) ON DELETE SET NULL,
    env_tag           TEXT NOT NULL DEFAULT 'dev',
    tab_color         TEXT NOT NULL DEFAULT '#2D7D9A',
    is_temporary      INTEGER NOT NULL DEFAULT 0,
    last_connected_at DATETIME,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_hosts_last_connected ON hosts(last_connected_at DESC);

CREATE TABLE IF NOT EXISTS tunnels (
    id          TEXT PRIMARY KEY,
    host_id     TEXT NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    direction   TEXT NOT NULL CHECK (direction IN ('L', 'R', 'D')),
    local_addr  TEXT NOT NULL,
    remote_addr TEXT,
    auto_start  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_tunnels_host_id ON tunnels(host_id);
