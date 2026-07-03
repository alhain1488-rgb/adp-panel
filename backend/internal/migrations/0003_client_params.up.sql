-- Per-client protocol data as JSON (additive). WireGuard/AmneziaWG needs a
-- per-client keypair; it lives here so adding the protocol keeps the schema
-- shape (SPEC invariant: protocol params live in JSON columns).
ALTER TABLE clients ADD COLUMN params_json TEXT NOT NULL DEFAULT '{}';
