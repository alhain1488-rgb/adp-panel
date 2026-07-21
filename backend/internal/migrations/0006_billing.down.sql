DROP INDEX IF EXISTS idx_billing_tx_charge;
DROP INDEX IF EXISTS idx_billing_tx_client;
DROP TABLE IF EXISTS billing_transactions;
ALTER TABLE clients DROP COLUMN billing_managed;
ALTER TABLE clients DROP COLUMN active_until;
ALTER TABLE clients DROP COLUMN wallet_kopecks;
