-- +goose Up
-- +goose StatementBegin
-- Index payments.called_contract (tx.to).
--
-- The classification view payment_classified_v1 LEFT JOINs the contamination
-- denylist on (chain, called_contract):
--
--   LEFT JOIN deny d ON d.chain = p.chain AND d.called_contract = p.called_contract
--
-- Without this index that join drives a full sequential scan of payments
-- (64M+ rows) on every read of the view. The existing principal indexes cover
-- facilitator/payer/payee but not the contract dimension that the denylist
-- precedence rule keys on. Matches the (chain, col) shape used by
-- idx_payments_block / idx_payments_timestamp.
CREATE INDEX IF NOT EXISTS idx_payments_called_contract
    ON payments(chain, called_contract);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_payments_called_contract;
-- +goose StatementEnd
