-- Optional contact e-mail for a client (additive). Used to send the client their
-- subscription link, and lets the panel reach subscribers directly.
ALTER TABLE clients ADD COLUMN email TEXT NOT NULL DEFAULT '';
