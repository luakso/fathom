package base

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// We test the Client wrapper indirectly via the Tailer (Task 4). This file's
// purpose is the contract test: any future Client implementation must satisfy
// the interface and return the documented zero values on the documented
// error conditions.

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
