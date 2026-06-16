-- +goose Up
-- +goose StatementBegin
-- v2 Plan 3 (cancellations slice): index ERC-3009 AuthorizationCanceled events.
-- A cancellation is an authorization the payer signed and then canceled
-- (cancelAuthorization) BEFORE it was ever used — an abandonment / reliability
-- signal. There is no companion Transfer and no payment row; these go to their
-- own light table. Standalone table: nothing SELECT p.* depends on it, so unlike
-- 00011/00012 there is NO view-recreate. nonce is BYTEA (raw 32 bytes) to match
-- payments.auth_nonce; addresses are TEXT lowercase-hex like payments.payer.
-- The payer_account_type half of the original Plan 3 is split out (needs an RPC
-- subsystem the collector does not have). See
-- docs/superpowers/specs/2026-06-16-authorization-cancellations-design.md.
CREATE TABLE IF NOT EXISTS authorization_cancellations (
    chain            TEXT        NOT NULL,
    tx_hash          TEXT        NOT NULL,
    log_index        INTEGER     NOT NULL,
    authorizer       TEXT        NOT NULL,
    nonce            BYTEA       NOT NULL,
    block_number     BIGINT      NOT NULL,
    block_time       TIMESTAMPTZ NOT NULL,
    transaction_from TEXT        NOT NULL,
    PRIMARY KEY (chain, tx_hash, log_index)
);
CREATE INDEX IF NOT EXISTS idx_auth_cancel_authorizer ON authorization_cancellations (authorizer);
CREATE INDEX IF NOT EXISTS idx_auth_cancel_block_time ON authorization_cancellations (block_time);

-- Read view, created in-migration so a migrations-only DB (goose.Up, the test
-- setup) has it — mirroring how 00011 creates payment_x402_v1. Kept byte-identical
-- to database/views/authorization_cancellation_v1.sql, which init-db re-applies.
CREATE OR REPLACE VIEW authorization_cancellation_v1 AS
WITH allow AS (
    SELECT chain, address
    FROM facilitator_allowlist
    WHERE since_version <= 1 AND (until_version IS NULL OR until_version > 1)
)
SELECT
    c.*,
    (a.address IS NOT NULL) AS facilitator_known,
    1 AS methodology_version
FROM authorization_cancellations c
LEFT JOIN allow a ON a.chain = c.chain AND a.address = c.transaction_from;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP VIEW IF EXISTS authorization_cancellation_v1;
DROP TABLE IF EXISTS authorization_cancellations;
-- +goose StatementEnd
