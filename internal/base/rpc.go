package base

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// Client is the RPC surface base-collector's live tail needs. Kept small for
// testability — fakeRPC in tailer_fetch_test.go (Task 3) is the canonical fake.
type Client interface {
	// BlockNumber returns the current chain head.
	BlockNumber(ctx context.Context) (uint64, error)
	// FilterLogs returns logs matching the given filter.
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)
	// BlockByNumber returns one block including its transactions (we need the
	// full input bytes for the sighash filter).
	BlockByNumber(ctx context.Context, number uint64) (*types.Block, error)
	// BlockReceipts maps to eth_getBlockReceipts: one round-trip per block
	// vs. N for per-tx receipts. Slightly more bytes; dramatically fewer
	// requests — net win on dense blocks (spec §8).
	BlockReceipts(ctx context.Context, number uint64) ([]*types.Receipt, error)
	// Close releases the underlying connection.
	Close()
}

// RPCClient wraps go-ethereum's ethclient.Client.
// ethclient.Client already exposes eth_getBlockReceipts via BlockReceipts, so
// no separate raw rpc.Client is needed.
type RPCClient struct {
	eth *ethclient.Client
}

// NewRPCClient dials the URL and returns a Client. URL must be non-empty.
func NewRPCClient(url string) (*RPCClient, error) {
	if url == "" {
		return nil, errors.New("rpc url is empty")
	}
	rc, err := rpc.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rpc dial %q: %w", url, err)
	}
	return &RPCClient{eth: ethclient.NewClient(rc)}, nil
}

// BlockNumber returns the current chain head number.
func (c *RPCClient) BlockNumber(ctx context.Context) (uint64, error) {
	return c.eth.BlockNumber(ctx)
}

// FilterLogs returns logs matching the given filter query.
func (c *RPCClient) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	return c.eth.FilterLogs(ctx, q)
}

// BlockByNumber returns the block at the given number including full transactions.
func (c *RPCClient) BlockByNumber(ctx context.Context, number uint64) (*types.Block, error) {
	return c.eth.BlockByNumber(ctx, new(big.Int).SetUint64(number))
}

// BlockReceipts fetches all receipts for the block at the given number via
// eth_getBlockReceipts. In go-ethereum v1.17.3 ethclient.Client exposes this
// directly via BlockReceipts(ctx, rpc.BlockNumberOrHash).
func (c *RPCClient) BlockReceipts(ctx context.Context, number uint64) ([]*types.Receipt, error) {
	bnh := rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(number)) //nolint:gosec // uint64→int64 safe for block numbers
	receipts, err := c.eth.BlockReceipts(ctx, bnh)
	if err != nil {
		return nil, fmt.Errorf("eth_getBlockReceipts %d: %w", number, err)
	}
	return receipts, nil
}

// Close releases the underlying RPC connection.
func (c *RPCClient) Close() {
	c.eth.Close()
}
