-- Initial schema (SPEC §4). Protocol-specific parameters live in JSON columns;
-- adding a protocol never requires a schema change.

CREATE TABLE admins (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    username       TEXT NOT NULL UNIQUE,
    password_hash  TEXT NOT NULL,
    totp_secret_enc TEXT NOT NULL DEFAULT '',
    totp_enabled   INTEGER NOT NULL DEFAULT 0,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE servers (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    name                  TEXT NOT NULL,
    host                  TEXT NOT NULL,
    ssh_port              INTEGER NOT NULL DEFAULT 22,
    ssh_user              TEXT NOT NULL DEFAULT 'root',
    ssh_auth_method       TEXT NOT NULL DEFAULT 'key',   -- 'key' | 'password'
    ssh_secret_enc        TEXT NOT NULL DEFAULT '',
    ssh_passphrase_enc    TEXT NOT NULL DEFAULT '',
    xray_config_path      TEXT NOT NULL DEFAULT '/usr/local/etc/xray/config.json',
    xray_service_name     TEXT NOT NULL DEFAULT 'xray',
    hysteria_config_path  TEXT NOT NULL DEFAULT '/etc/sing-box/config.json',
    hysteria_service_name TEXT NOT NULL DEFAULT 'sing-box',
    ip                    TEXT NOT NULL DEFAULT '',
    geo_country           TEXT NOT NULL DEFAULT '',
    geo_city              TEXT NOT NULL DEFAULT '',
    geo_asn               TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL DEFAULT 'unknown', -- online|offline|unknown|error
    provision_status      TEXT NOT NULL DEFAULT 'pending', -- pending|installing|installed|failed
    provision_error       TEXT NOT NULL DEFAULT '',
    last_check_at         DATETIME,
    last_sync_at          DATETIME,
    last_sync_error       TEXT NOT NULL DEFAULT '',
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE inbounds (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id            INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    tag                  TEXT NOT NULL,
    protocol             TEXT NOT NULL,  -- vless|vmess|trojan|shadowsocks|hysteria2|...
    listen               TEXT NOT NULL DEFAULT '0.0.0.0',
    port                 INTEGER NOT NULL,
    settings_json        TEXT NOT NULL DEFAULT '{}',
    stream_settings_json TEXT NOT NULL DEFAULT '{}',
    sniffing_json        TEXT NOT NULL DEFAULT '{}',
    remark               TEXT NOT NULL DEFAULT '',
    enabled              INTEGER NOT NULL DEFAULT 1,
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, tag)
);
CREATE INDEX idx_inbounds_server ON inbounds(server_id);

CREATE TABLE clients (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL,
    uuid               TEXT NOT NULL UNIQUE,
    password           TEXT NOT NULL,
    subscription_token TEXT NOT NULL UNIQUE,
    enabled            INTEGER NOT NULL DEFAULT 1,
    remark             TEXT NOT NULL DEFAULT '',
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE client_inbounds (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    client_id  INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    inbound_id INTEGER NOT NULL REFERENCES inbounds(id) ON DELETE CASCADE,
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(client_id, inbound_id)
);
CREATE INDEX idx_client_inbounds_client ON client_inbounds(client_id);
CREATE INDEX idx_client_inbounds_inbound ON client_inbounds(inbound_id);

CREATE TABLE audit_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    admin_id    INTEGER REFERENCES admins(id) ON DELETE SET NULL,
    action      TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT '',
    target_id   INTEGER NOT NULL DEFAULT 0,
    detail_json TEXT NOT NULL DEFAULT '{}',
    ip          TEXT NOT NULL DEFAULT '',
    user_agent  TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_audit_created ON audit_logs(created_at);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);
