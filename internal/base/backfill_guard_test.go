package base_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lukostrobl/fathom/internal/base"
)

// candidateNoCompanionBatch builds a batch with one genuine x402 candidate
// (AuthorizationUsed on USDC, parent tx carries a kept selector) but NO
// companion Transfer log — exactly the shape a JoinAll/pairing regression would
// produce: candidates present, zero rows assembled.
func candidateNoCompanionBatch() base.HyperSyncBatch {
	b := fixtureBatch()
	// Drop the companion Transfer (index 1), keep the AuthorizationUsed (index 0).
	b.Data.Logs = b.Data.Logs[:1]
	return b
}

func TestBackfill_Run_HaltsWhenAllCandidatesDrop(t *testing.T) {
	// Store with a nil pool: the guard must fire BEFORE InsertBatch, so the
	// store is never touched. If the guard regresses, this nil-deref panics —
	// a loud, useful failure.
	store := base.NewStore(nil)
	f := &fakeFetcher{batches: []base.HyperSyncBatch{candidateNoCompanionBatch()}}
	bf := base.NewBackfiller(f, store)

	err := bf.Run(context.Background(), 100, 100)
	require.Error(t, err, "a batch with candidates but zero kept rows must halt, not silently advance")
	require.Contains(t, strings.ToLower(err.Error()), "0 rows")
}
