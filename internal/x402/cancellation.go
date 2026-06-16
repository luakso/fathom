package x402

import (
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Cancellation is one ERC-3009 AuthorizationCanceled event: a payer canceled a
// signed authorization nonce before it was ever used. It carries no amount or
// payee (the authorization was never consumed) — only the authorizer, the
// canceled nonce, and the submitter of the cancel transaction. Stored in its
// own table; it is NOT a payment.
type Cancellation struct {
	Chain           string    // ChainBase
	TxHash          string    // lowercase 0x hex
	LogIndex        uint32    // position in the block
	Authorizer      string    // lowercase 0x hex — the payer who canceled
	Nonce           []byte    // 32-byte authorization nonce (raw)
	BlockNumber     uint64    //
	BlockTime       time.Time // UTC
	TransactionFrom string    // lowercase 0x hex — cancel submitter (facilitator-or-self)
}

// DecodeAuthorizationCanceled extracts (authorizer, nonce) from an EIP-3009
// AuthorizationCanceled log:
//
//	AuthorizationCanceled(address indexed authorizer, bytes32 indexed nonce)
//
// Both params are INDEXED, so the log has 3 topics (signature, authorizer,
// nonce) and empty data — identical shape to AuthorizationUsed. Caller is
// responsible for confirming Topics[0] == AuthorizationCanceledTopic and
// Address == USDCProxyBase. Returned nonce is a copy — safe to store.
func DecodeAuthorizationCanceled(log Log) (authorizer common.Address, nonce []byte, err error) {
	if len(log.Topics) != 3 {
		return common.Address{}, nil,
			fmt.Errorf("authorization-canceled log: expected 3 topics, got %d", len(log.Topics))
	}
	authorizer = common.BytesToAddress(log.Topics[1].Bytes())
	nonce = make([]byte, 32)
	copy(nonce, log.Topics[2].Bytes())
	return authorizer, nonce, nil
}

// ExtractCancellations scans the batch's logs for AuthorizationCanceled-on-USDC
// events and builds a Cancellation per log. It mirrors Assemble's lookups:
// parent tx (for the submitter) and block (for the timestamp). A log whose
// parent tx or block is absent, or whose timestamp overflows int64, is logged
// and skipped — never aborts the batch (matches the Assemble per-row policy).
//
// Output is in input-log order.
func ExtractCancellations(
	logs []Log,
	txByHash map[common.Hash]Transaction,
	blockByNumber map[uint64]Block,
) []Cancellation {
	out := make([]Cancellation, 0)
	for _, lg := range logs {
		if lg.Address != USDCProxyBase {
			continue
		}
		if len(lg.Topics) == 0 || lg.Topics[0] != AuthorizationCanceledTopic {
			continue
		}
		authorizer, nonce, err := DecodeAuthorizationCanceled(lg)
		if err != nil {
			slog.Warn("extract-cancellations: decode failed", "tx_hash", lg.TxHash.Hex(), "err", err)
			continue
		}
		tx, ok := txByHash[lg.TxHash]
		if !ok {
			slog.Warn("extract-cancellations: missing parent tx", "tx_hash", lg.TxHash.Hex(), "log_index", lg.LogIndex)
			continue
		}
		block, ok := blockByNumber[lg.BlockNumber]
		if !ok {
			slog.Warn("extract-cancellations: missing block context", "tx_hash", lg.TxHash.Hex(), "block_number", lg.BlockNumber)
			continue
		}
		if block.Timestamp > math.MaxInt64 {
			slog.Warn("extract-cancellations: block timestamp overflows int64", "tx_hash", lg.TxHash.Hex(), "timestamp", block.Timestamp)
			continue
		}
		out = append(out, Cancellation{
			Chain:           ChainBase,
			TxHash:          strings.ToLower(lg.TxHash.Hex()),
			LogIndex:        lg.LogIndex,
			Authorizer:      strings.ToLower(authorizer.Hex()),
			Nonce:           nonce,
			BlockNumber:     lg.BlockNumber,
			BlockTime:       time.Unix(int64(block.Timestamp), 0).UTC(), //nolint:gosec // bounds-checked above
			TransactionFrom: strings.ToLower(tx.From.Hex()),
		})
	}
	return out
}
