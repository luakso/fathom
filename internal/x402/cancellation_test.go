package x402

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func canceledLog(authorizer common.Address, nonce common.Hash, txHash common.Hash, blockNum uint64, logIdx uint32) Log {
	return Log{
		Address:     USDCProxyBase,
		Topics:      []common.Hash{AuthorizationCanceledTopic, common.BytesToHash(authorizer.Bytes()), nonce},
		BlockNumber: blockNum,
		TxHash:      txHash,
		LogIndex:    logIdx,
	}
}

func TestDecodeAuthorizationCanceled(t *testing.T) {
	t.Parallel()
	authorizer := common.HexToAddress("0x00000000000000000000000000000000000000aa")
	nonce := common.HexToHash("0xabcdef")
	lg := canceledLog(authorizer, nonce, common.HexToHash("0x01"), 10, 3)

	gotAuth, gotNonce, err := DecodeAuthorizationCanceled(lg)
	require.NoError(t, err)
	require.Equal(t, authorizer, gotAuth)
	require.Equal(t, nonce.Bytes(), gotNonce)
	require.Len(t, gotNonce, 32)
}

func TestDecodeAuthorizationCanceled_WrongTopicCount(t *testing.T) {
	t.Parallel()
	lg := Log{Address: USDCProxyBase, Topics: []common.Hash{AuthorizationCanceledTopic}}
	_, _, err := DecodeAuthorizationCanceled(lg)
	require.Error(t, err)
}

func TestExtractCancellations(t *testing.T) {
	t.Parallel()
	authorizer := common.HexToAddress("0x00000000000000000000000000000000000000aa")
	submitter := common.HexToAddress("0x00000000000000000000000000000000000000fa")
	txHash := common.HexToHash("0xdead")
	nonce := common.HexToHash("0xbeef")

	logs := []Log{
		canceledLog(authorizer, nonce, txHash, 100, 7),
		{Address: USDCProxyBase, Topics: []common.Hash{AuthorizationUsedTopic}, TxHash: txHash, BlockNumber: 100, LogIndex: 8},
	}
	txByHash := map[common.Hash]Transaction{
		txHash: {Hash: txHash, From: submitter, BlockNumber: 100},
	}
	blockByNumber := map[uint64]Block{
		100: {Number: 100, Timestamp: 1_700_000_000},
	}

	got := ExtractCancellations(logs, txByHash, blockByNumber)
	require.Len(t, got, 1)
	c := got[0]
	require.Equal(t, ChainBase, c.Chain)
	require.Equal(t, strings.ToLower(txHash.Hex()), c.TxHash) // full 32-byte 0x hex, mirrors Assemble's Payment.TxHash
	require.Equal(t, uint32(7), c.LogIndex)
	require.Equal(t, "0x00000000000000000000000000000000000000aa", c.Authorizer)
	require.Equal(t, "0x00000000000000000000000000000000000000fa", c.TransactionFrom)
	require.Equal(t, nonce.Bytes(), c.Nonce)
	require.Equal(t, uint64(100), c.BlockNumber)
	require.Equal(t, int64(1_700_000_000), c.BlockTime.Unix())
}

func TestExtractCancellations_SkipsWhenTxOrBlockMissing(t *testing.T) {
	t.Parallel()
	authorizer := common.HexToAddress("0x00000000000000000000000000000000000000aa")
	txHash := common.HexToHash("0xdead")
	logs := []Log{canceledLog(authorizer, common.HexToHash("0x1"), txHash, 100, 7)}

	require.Empty(t, ExtractCancellations(logs, map[common.Hash]Transaction{}, map[uint64]Block{}))

	txOnly := map[common.Hash]Transaction{txHash: {Hash: txHash, BlockNumber: 100}}
	require.Empty(t, ExtractCancellations(logs, txOnly, map[uint64]Block{}))
}

func TestExtractCancellations_IgnoresNonUSDCAddress(t *testing.T) {
	t.Parallel()
	lg := canceledLog(common.HexToAddress("0xaa"), common.HexToHash("0x1"), common.HexToHash("0xd"), 100, 7)
	lg.Address = common.HexToAddress("0x000000000000000000000000000000000000dead") // not USDC
	txByHash := map[common.Hash]Transaction{lg.TxHash: {Hash: lg.TxHash, BlockNumber: 100}}
	blockByNumber := map[uint64]Block{100: {Number: 100, Timestamp: 1}}
	require.Empty(t, ExtractCancellations([]Log{lg}, txByHash, blockByNumber))
}
