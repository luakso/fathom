package base

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The concrete RPCClient is exercised through a fake Client in the FetchRange
// and Tailer tests. This file holds only the contract checks that don't need a
// live RPC: the compile-time interface assertion and the empty-URL guard.

func TestClient_InterfaceShape(t *testing.T) {
	t.Parallel()
	// Compile-time assertion that *RPCClient implements Client.
	var _ Client = (*RPCClient)(nil)
}

func TestNewRPCClient_RejectsEmptyURL(t *testing.T) {
	t.Parallel()
	_, err := NewRPCClient("")
	require.Error(t, err)
}
