//go:build integration

package metrics_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lukostrobl/fathom/internal/metrics"
)

// Windowed-subset latency, expired/not-yet-valid counts, and the all==known+unknown
// reconciliation, all computed on a tiny hand-checkable fixture anchored to 2026-06-10.
func TestRebuildReliability_WindowStats(t *testing.T) {
	ctx, db, pool := setupMetrics(t)
	allowlist(t, ctx, db, "0xfac1") // known
	seedWindowedPayments(t, ctx, db, []seedWindowedRow{
		// known: fast 5s latency, valid window, settled inside it
		{"0xa", 0, "2026-06-10T10:00:05Z", "0xfac1", "0xp1", "0xs1", "1.00", "2026-06-10T10:00:00Z", "2026-06-10T11:00:00Z"},
		// known: slow 2h latency (7200s → >10m bucket), settled AFTER valid_before (expired)
		{"0xb", 0, "2026-06-10T12:00:00Z", "0xfac1", "0xp2", "0xs1", "2.00", "2026-06-10T10:00:00Z", "2026-06-10T11:00:00Z"},
		// known: window-less (NULL window) — counts as a settlement, excluded from windowed/latency
		{"0xc", 0, "2026-06-10T13:00:00Z", "0xfac1", "0xp3", "0xs1", "3.00", "", ""},
		// unknown: not-yet-valid (settled BEFORE valid_after)
		{"0xd", 0, "2026-06-10T09:00:00Z", "0xfac2", "0xp4", "0xs2", "4.00", "2026-06-10T09:30:00Z", "2026-06-10T11:00:00Z"},
	})

	require.NoError(t, metrics.Rebuild(ctx, pool, testPrices(t)))

	var settle, windowed, expired, notYet int64
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT settlement_count, windowed_count, expired_count, not_yet_valid_count
		FROM metrics_reliability_window_v2 WHERE window_name='all' AND membership='all'`).
		Scan(&settle, &windowed, &expired, &notYet))
	require.Equal(t, int64(4), settle, "all four payments are settlements")
	require.Equal(t, int64(3), windowed, "three carry a full auth window (0xc is window-less)")
	require.Equal(t, int64(1), expired, "0xb settled after valid_before")
	require.Equal(t, int64(1), notYet, "0xd settled before valid_after")

	var knownS, unknownS int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT settlement_count FROM metrics_reliability_window_v2 WHERE window_name='all' AND membership='known'`).Scan(&knownS))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT settlement_count FROM metrics_reliability_window_v2 WHERE window_name='all' AND membership='unknown'`).Scan(&unknownS))
	require.Equal(t, settle, knownS+unknownS, "membership must reconcile to 'all'")
	require.Equal(t, int64(3), knownS)
	require.Equal(t, int64(1), unknownS)

	var sub1, b110s, gt10m int64
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT lat_bucket_sub1s, lat_bucket_1_10s, lat_bucket_gt10m
		FROM metrics_reliability_window_v2 WHERE window_name='all' AND membership='known'`).
		Scan(&sub1, &b110s, &gt10m))
	require.Equal(t, int64(0), sub1)
	require.Equal(t, int64(1), b110s)
	require.Equal(t, int64(1), gt10m)
}

func TestRebuildReliability_Daily(t *testing.T) {
	ctx, db, pool := setupMetrics(t)
	allowlist(t, ctx, db, "0xfac1")
	seedWindowedPayments(t, ctx, db, []seedWindowedRow{
		{"0xa", 0, "2026-06-09T10:00:00Z", "0xfac2", "0xp1", "0xs1", "1.00", "2026-06-09T09:00:00Z", "2026-06-09T11:00:00Z"},
		{"0xb", 0, "2026-06-10T10:00:00Z", "0xfac2", "0xp2", "0xs1", "2.00", "2026-06-10T09:00:00Z", "2026-06-10T11:00:00Z"},
	})
	seedCancellations(t, ctx, db, []seedCancelRow{
		{"0xc1", 0, "0xp2", "2026-06-10T12:00:00Z", "0xrelayer"},
	})

	require.NoError(t, metrics.Rebuild(ctx, pool, testPrices(t)))

	var day1 int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT settlement_count FROM metrics_reliability_daily_v2 WHERE day='2026-06-09' AND membership='unknown'`).Scan(&day1))
	require.Equal(t, int64(1), day1)

	var cancel int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT cancellation_count FROM metrics_reliability_daily_v2 WHERE day='2026-06-10' AND membership='unknown'`).Scan(&cancel))
	require.Equal(t, int64(1), cancel, "the 2026-06-10 cancellation joins to that day's unknown row")

	var dailySum, windowAll int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT coalesce(sum(settlement_count),0) FROM metrics_reliability_daily_v2`).Scan(&dailySum))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT settlement_count FROM metrics_reliability_window_v2 WHERE window_name='all' AND membership='all'`).Scan(&windowAll))
	require.Equal(t, windowAll, dailySum, "daily settlements must sum to the all-window total")
}
