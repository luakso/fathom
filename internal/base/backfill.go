package base

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/lukostrobl/fathom/internal/x402"
)

// Backfiller drives the HyperSync stream → decode → assemble → store loop.
//
// One Run pass covers the [fromBlock, toBlock] window inclusive. Run is safe
// to invoke repeatedly with overlapping ranges: idempotency comes from the
// payments PK and the cursor's monotonic advance.
//
// Per spec §11: on any fetch/decode/insert error, Run returns the error and
// the caller exits non-zero. Re-invoking from the last committed cursor
// resumes cleanly — the cursor only advances on a successfully committed
// batch.
type Backfiller struct {
	fetcher Fetcher
	store   *Store
}

// NewBackfiller constructs a Backfiller. Both dependencies are required.
func NewBackfiller(fetcher Fetcher, store *Store) *Backfiller {
	return &Backfiller{fetcher: fetcher, store: store}
}

// Run streams batches from fromBlock to toBlock (inclusive) and writes them
// to Store. Returns the first error encountered; ctx cancellation triggers
// a graceful shutdown between batches (never mid-batch).
func (b *Backfiller) Run(ctx context.Context, fromBlock, toBlock uint64) error {
	q := BuildBackfillQuery(fromBlock, toBlock)
	stream, err := b.fetcher.Stream(q)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	defer func() { _ = stream.Close() }()

	for {
		select {
		case <-ctx.Done():
			slog.Info("backfill: shutdown requested between batches", "err", ctx.Err())
			return nil
		default:
		}

		started := time.Now()
		batch, ok, err := stream.Next()
		if err != nil {
			return fmt.Errorf("stream next: %w", err)
		}
		if !ok {
			slog.Info("backfill: stream complete")
			return nil
		}

		payments, decodeErr := decodeBatch(batch)
		if decodeErr != nil {
			return fmt.Errorf("decode batch: %w", decodeErr)
		}

		maxBlock := batch.MaxBlock() // returns 0 for empty batches → cursor skip in Store
		if err := b.store.InsertBatch(ctx, payments, maxBlock); err != nil {
			return fmt.Errorf("insert batch (rows=%d max_block=%d): %w", len(payments), maxBlock, err)
		}

		slog.Info(
			"backfill: batch committed",
			"rows", len(payments),
			"max_block", maxBlock,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	}
}

// decodeBatch converts the HyperSync wire batch into ([]Payment) ready for
// Store.InsertBatch. Per-row decode failures (bad hex, missing companion)
// log a warn inside Assemble and are skipped — only structural failures
// (whole-row convert errors) abort.
func decodeBatch(batch HyperSyncBatch) ([]x402.Payment, error) {
	logs := make([]x402.Log, 0, len(batch.Data.Logs))
	receiptByHash := map[common.Hash][]x402.Log{}
	for i, hl := range batch.Data.Logs {
		lg, err := ConvertLog(hl)
		if err != nil {
			return nil, fmt.Errorf("log[%d]: %w", i, err)
		}
		logs = append(logs, lg)
		receiptByHash[lg.TxHash] = append(receiptByHash[lg.TxHash], lg)
	}

	txByHash := map[common.Hash]x402.Transaction{}
	for i, ht := range batch.Data.Transactions {
		tx, err := ConvertTransaction(ht)
		if err != nil {
			return nil, fmt.Errorf("tx[%d]: %w", i, err)
		}
		txByHash[tx.Hash] = tx
	}

	blockByNumber := map[uint64]x402.Block{}
	for i, hb := range batch.Data.Blocks {
		blk, err := ConvertBlock(hb)
		if err != nil {
			return nil, fmt.Errorf("block[%d]: %w", i, err)
		}
		blockByNumber[blk.Number] = blk
	}

	return x402.Assemble(logs, txByHash, receiptByHash, blockByNumber), nil
}
