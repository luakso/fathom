-- +goose Up
-- +goose StatementBegin
-- Make amount_usdc a GENERATED column derived from amount_raw.
--
-- amount_usdc is, by definition, amount_raw / 10^6 (USDC has 6 decimals). It
-- was being computed in Go and written as an independent column, which means it
-- could silently drift from amount_raw on any code change. A generated column
-- makes the database the single source of truth and removes the redundant write.
--
-- We MULTIPLY by 0.000001 rather than divide by 1000000: Postgres numeric
-- division targets ~16 significant digits, so for a large amount_raw the
-- quotient is rounded to scale 0 (dropping the cents) BEFORE the cast can fix
-- it. Multiplication's result scale is the sum of operand scales (0 + 6 = 6),
-- so amount_raw * 0.000001 is exact; the (38,6) cast preserves type and range.
--
-- Postgres has no in-place "convert existing column to generated", so this drops
-- and re-adds the column. ADD COLUMN ... GENERATED ... STORED REWRITES THE TABLE
-- under an ACCESS EXCLUSIVE lock — on the current 64M+ rows this takes minutes
-- and blocks reads/writes for its duration. Run it in a maintenance window with
-- the collector paused. (Postgres lacks VIRTUAL generated columns before 18, so
-- a no-rewrite variant is not available here.)
--
-- payment_classified_v1 is `SELECT p.*`, which freezes an explicit reference to
-- amount_usdc at view-creation time, so DROP COLUMN is blocked until the view is
-- gone. We drop it first and recreate it byte-for-byte from
-- database/views/payment_classified_v1.sql at the end, keeping `goose up` alone
-- a fully working state (init-db re-applies the same view with CREATE OR REPLACE,
-- a no-op match). The recreated view now also picks up the columns added since
-- it was first created (max_fee_per_gas/max_priority_fee_per_gas from 00007 and
-- amount_usdc in its new trailing position).
DROP VIEW IF EXISTS payment_classified_v1;

ALTER TABLE payments DROP COLUMN amount_usdc;
ALTER TABLE payments
    ADD COLUMN amount_usdc NUMERIC(38,6)
    GENERATED ALWAYS AS ((amount_raw * 0.000001)::numeric(38,6)) STORED;

CREATE OR REPLACE VIEW payment_classified_v1 AS
WITH allow AS (
    SELECT chain, address
    FROM facilitator_allowlist
    WHERE since_version <= 1 AND (until_version IS NULL OR until_version > 1)
),
deny AS (
    SELECT chain, called_contract
    FROM contamination_denylist
    WHERE since_version <= 1 AND (until_version IS NULL OR until_version > 1)
)
SELECT
    p.*,
    CASE
        WHEN d.called_contract IS NOT NULL THEN 'contamination'
        WHEN a.address         IS NOT NULL THEN 'agentic'
        ELSE 'contested'
    END AS attribution,
    1 AS methodology_version
FROM payments p
LEFT JOIN deny  d ON d.chain = p.chain AND d.called_contract = p.called_contract
LEFT JOIN allow a ON a.chain = p.chain AND a.address = p.facilitator;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Restore amount_usdc as a plain (non-generated) column, repopulated from
-- amount_raw so the down-migration leaves a consistent table. Writers (store.go
-- at this revision) must resume supplying the value themselves. The dependent
-- view is dropped and recreated for the same reason as the up-migration.
DROP VIEW IF EXISTS payment_classified_v1;

ALTER TABLE payments DROP COLUMN amount_usdc;
ALTER TABLE payments ADD COLUMN amount_usdc NUMERIC(38,6) NOT NULL DEFAULT 0;
ALTER TABLE payments ALTER COLUMN amount_usdc DROP DEFAULT;
UPDATE payments SET amount_usdc = (amount_raw * 0.000001)::numeric(38,6);

CREATE OR REPLACE VIEW payment_classified_v1 AS
WITH allow AS (
    SELECT chain, address
    FROM facilitator_allowlist
    WHERE since_version <= 1 AND (until_version IS NULL OR until_version > 1)
),
deny AS (
    SELECT chain, called_contract
    FROM contamination_denylist
    WHERE since_version <= 1 AND (until_version IS NULL OR until_version > 1)
)
SELECT
    p.*,
    CASE
        WHEN d.called_contract IS NOT NULL THEN 'contamination'
        WHEN a.address         IS NOT NULL THEN 'agentic'
        ELSE 'contested'
    END AS attribution,
    1 AS methodology_version
FROM payments p
LEFT JOIN deny  d ON d.chain = p.chain AND d.called_contract = p.called_contract
LEFT JOIN allow a ON a.chain = p.chain AND a.address = p.facilitator;
-- +goose StatementEnd
