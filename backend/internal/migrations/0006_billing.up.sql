-- Billing (additive). Turns the panel from personal-only into an optional paid
-- service: each client gets a prepaid wallet (in kopecks, 100 = 1 RUB) and a
-- subscription expiry. active_until = '' means the client never engaged with
-- billing (a manually-created client) and is never auto-suspended. A client
-- becomes billing_managed the first time they buy a subscription; only managed
-- clients are auto-suspended by the billing engine when their time runs out.
ALTER TABLE clients ADD COLUMN wallet_kopecks INTEGER NOT NULL DEFAULT 0;
ALTER TABLE clients ADD COLUMN active_until TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN billing_managed INTEGER NOT NULL DEFAULT 0;

-- Append-only ledger of every wallet movement: Stars top-ups, subscription
-- purchases, operator adjustments, refunds. amount_kopecks is signed (+credit,
-- -debit). charge_id holds the Telegram payment charge id for Stars top-ups,
-- used both for refunds and for idempotent crediting (a partial unique index
-- guarantees the same payment is never credited twice).
CREATE TABLE billing_transactions (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    client_id      INTEGER NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    kind           TEXT NOT NULL,            -- topup | purchase | adjust | refund
    method         TEXT NOT NULL DEFAULT '', -- stars | card | sbp | crypto | manual
    amount_kopecks INTEGER NOT NULL,         -- signed: +credit, -debit
    stars          INTEGER NOT NULL DEFAULT 0,
    tariff         TEXT NOT NULL DEFAULT '',  -- week | month | year (purchases)
    detail         TEXT NOT NULL DEFAULT '',
    charge_id      TEXT NOT NULL DEFAULT '',  -- telegram_payment_charge_id (Stars)
    created_at     TEXT NOT NULL
);

CREATE INDEX idx_billing_tx_client ON billing_transactions(client_id, id DESC);
CREATE UNIQUE INDEX idx_billing_tx_charge ON billing_transactions(charge_id) WHERE charge_id != '';
