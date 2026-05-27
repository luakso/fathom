package base

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"github.com/lukostrobl/fathom/internal/x402"
)

// ConvertLog turns one HyperSyncLog into an x402.Log. Errors on malformed
// hex inputs; the caller is expected to skip that row and continue the batch.
func ConvertLog(in HyperSyncLog) (x402.Log, error) {
	addr, err := parseAddress(in.Address)
	if err != nil {
		return x402.Log{}, fmt.Errorf("log.address: %w", err)
	}
	topics := make([]common.Hash, 0, len(in.Topics))
	for i, t := range in.Topics {
		h, err := parseHash(t)
		if err != nil {
			return x402.Log{}, fmt.Errorf("log.topics[%d]: %w", i, err)
		}
		topics = append(topics, h)
	}
	data, err := hexBytes(in.Data)
	if err != nil {
		return x402.Log{}, fmt.Errorf("log.data: %w", err)
	}
	txHash, err := parseHash(in.TxHash)
	if err != nil {
		return x402.Log{}, fmt.Errorf("log.tx_hash: %w", err)
	}
	return x402.Log{
		Address:     addr,
		Topics:      topics,
		Data:        data,
		BlockNumber: in.BlockNumber,
		TxHash:      txHash,
		TxIndex:     in.TxIndex,
		LogIndex:    in.LogIndex,
	}, nil
}

// ConvertTransaction turns one HyperSyncTransaction into an x402.Transaction.
// BaseFeePerGas is nil for legacy (pre-EIP-1559) txs that have no base fee
// in the wire response.
func ConvertTransaction(in HyperSyncTransaction) (x402.Transaction, error) {
	hash, err := parseHash(in.Hash)
	if err != nil {
		return x402.Transaction{}, fmt.Errorf("tx.hash: %w", err)
	}
	from, err := parseAddress(in.From)
	if err != nil {
		return x402.Transaction{}, fmt.Errorf("tx.from: %w", err)
	}
	to, err := parseAddress(in.To)
	if err != nil {
		return x402.Transaction{}, fmt.Errorf("tx.to: %w", err)
	}
	input, err := hexBytes(in.Input)
	if err != nil {
		return x402.Transaction{}, fmt.Errorf("tx.input: %w", err)
	}
	gasPrice, err := ParseHexInt(in.EffectiveGasPrice)
	if err != nil {
		return x402.Transaction{}, fmt.Errorf("tx.effective_gas_price: %w", err)
	}
	var baseFee *big.Int
	if in.BaseFeePerGas != "" {
		baseFee, err = ParseHexInt(in.BaseFeePerGas)
		if err != nil {
			return x402.Transaction{}, fmt.Errorf("tx.base_fee_per_gas: %w", err)
		}
	}
	return x402.Transaction{
		Hash:              hash,
		BlockNumber:       in.BlockNumber,
		From:              from,
		To:                to,
		Input:             input,
		Type:              in.Type,
		Nonce:             in.Nonce,
		GasUsed:           in.GasUsed,
		EffectiveGasPrice: gasPrice,
		BaseFeePerGas:     baseFee,
	}, nil
}

// ConvertBlock turns one HyperSyncBlock into an x402.Block.
func ConvertBlock(in HyperSyncBlock) (x402.Block, error) {
	hash, err := parseHash(in.Hash)
	if err != nil {
		return x402.Block{}, fmt.Errorf("block.hash: %w", err)
	}
	return x402.Block{
		Number:    in.Number,
		Timestamp: in.Timestamp,
		Hash:      hash,
	}, nil
}

// ParseHexInt parses a 0x-prefixed hex string as a *big.Int.
// Empty string returns 0.
func ParseHexInt(s string) (*big.Int, error) {
	if s == "" {
		return new(big.Int), nil
	}
	v, ok := new(big.Int).SetString(strings.TrimPrefix(s, "0x"), 16)
	if !ok {
		return nil, fmt.Errorf("parse hex int %q", s)
	}
	return v, nil
}

func parseAddress(s string) (common.Address, error) {
	if !strings.HasPrefix(s, "0x") || len(s) != 42 {
		return common.Address{}, fmt.Errorf("invalid address %q", s)
	}
	return common.HexToAddress(s), nil
}

func parseHash(s string) (common.Hash, error) {
	if !strings.HasPrefix(s, "0x") {
		return common.Hash{}, fmt.Errorf("invalid hash %q", s)
	}
	return common.HexToHash(s), nil
}

func hexBytes(s string) ([]byte, error) {
	return hex.DecodeString(strings.TrimPrefix(s, "0x"))
}
