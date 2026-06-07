package base

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lukostrobl/fathom/internal/x402"
)

// allCandidatesLost is the guard predicate: it must fire only when the batch
// carried genuine x402 candidates (AuthLogs beyond the expected Denied drops)
// yet produced zero rows — the signature of a pairing/JoinAll regression.
func TestAllCandidatesLost(t *testing.T) {
	tests := []struct {
		name  string
		stats x402.AssembleStats
		want  bool
	}{
		{"no logs at all", x402.AssembleStats{}, false},
		{"normal: all kept", x402.AssembleStats{AuthLogs: 3, Kept: 3}, false},
		{"partial loss still produced rows", x402.AssembleStats{AuthLogs: 3, Kept: 2, Dropped: 1}, false},
		{"denied-only is expected, not loss", x402.AssembleStats{AuthLogs: 2, Denied: 2}, false},
		{"candidates present, zero kept → halt", x402.AssembleStats{AuthLogs: 2, Kept: 0, Dropped: 2}, true},
		{"one denied, one dropped, zero kept → halt", x402.AssembleStats{AuthLogs: 2, Denied: 1, Dropped: 1}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, allCandidatesLost(tc.stats))
		})
	}
}
