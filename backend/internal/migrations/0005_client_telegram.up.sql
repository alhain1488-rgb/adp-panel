-- Per-client Telegram delivery (additive). A client links their Telegram by
-- opening a personal deep link (t.me/<bot>?start=<tg_link_token>) and pressing
-- Start; the bot then stores their numeric chat id (tg_chat_id) so the panel can
-- push the subscription config straight to them. tg_username is their @handle,
-- kept for display only.
ALTER TABLE clients ADD COLUMN tg_chat_id TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN tg_username TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN tg_link_token TEXT NOT NULL DEFAULT '';

-- Give existing clients a stable link token so their deep link works right away.
UPDATE clients SET tg_link_token = lower(hex(randomblob(12))) WHERE tg_link_token = '';
