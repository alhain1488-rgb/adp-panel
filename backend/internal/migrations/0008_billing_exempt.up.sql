-- Lifetime free access (additive). billing_exempt marks clients the billing
-- engine must never auto-suspend, no matter what their subscription says. Unlike
-- billing_managed (which flips to 1 on a client's first purchase and stays there),
-- this flag is the operator's explicit promise and survives any later purchase.
ALTER TABLE clients ADD COLUMN billing_exempt INTEGER NOT NULL DEFAULT 0;

-- Everyone who already exists when this migration runs was using the panel while
-- it was still personal-only, so they keep free access for good. Clients created
-- afterwards default to 0 and are billed normally.
UPDATE clients SET billing_exempt = 1;
