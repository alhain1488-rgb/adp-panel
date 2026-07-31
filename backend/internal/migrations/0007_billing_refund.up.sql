-- Stars refunds (additive). A Stars top-up may be refunded once, via Telegram's
-- refundStarPayment. refunded_at is stamped on the ORIGINAL top-up row, which is
-- what makes a second refund of the same payment impossible; the compensating
-- debit is appended as its own 'refund' row with an empty charge_id, so the
-- partial unique index on charge_id keeps guarding idempotent crediting.
ALTER TABLE billing_transactions ADD COLUMN refunded_at TEXT NOT NULL DEFAULT '';
