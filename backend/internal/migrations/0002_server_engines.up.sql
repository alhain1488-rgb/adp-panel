-- Cache the last-known engine status per server (populated at check/provision
-- time), so the API can return it without SSH-ing on every read.
ALTER TABLE servers ADD COLUMN engines_json TEXT NOT NULL DEFAULT '[]';
