// Package solana provides Solana blockchain client implementation.
package solana

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/rs/zerolog"
)

type Client struct {
	rpcClient  *rpc.Client
	wsEndpoint string
	logger     zerolog.Logger
}

func NewClient(rpcEndpoint, wsEndpoint string, logger zerolog.Logger) *Client {
	return &Client{
		rpcClient:  rpc.New(rpcEndpoint),
		wsEndpoint: wsEndpoint,
		logger:     logger,
	}
}

func (c *Client) GetBalance(pubkey solana.PublicKey) (uint64, error) {
	result, err := c.rpcClient.GetBalance(context.Background(), pubkey, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("get balance: %w", err)
	}
	return result.Value, nil
}

func (c *Client) GetSlot() (uint64, error) {
	slot, err := c.rpcClient.GetSlot(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("get slot: %w", err)
	}
	return slot, nil
}

func (c *Client) SubmitTransaction(txBytes []byte) (string, error) {
	tx, err := solana.TransactionFromBytes(txBytes)
	if err != nil {
		return "", fmt.Errorf("decode transaction: %w", err)
	}
	sig, err := c.rpcClient.SendTransaction(context.Background(), tx)
	if err != nil {
		return "", fmt.Errorf("send transaction: %w", err)
	}
	return sig.String(), nil
}

func (c *Client) GetTransaction(signature string) (map[string]interface{}, error) {
	sig, err := solana.SignatureFromBase58(signature)
	if err != nil {
		return nil, fmt.Errorf("parse signature: %w", err)
	}

	result, err := c.rpcClient.GetTransaction(
		context.Background(),
		sig,
		&rpc.GetTransactionOpts{
			Encoding:   "jsonParsed",
			Commitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	return map[string]interface{}{
		"signature": signature,
		"slot":      result.Slot,
	}, nil
}

func (c *Client) WaitForConfirmation(signature string, timeout time.Duration) (bool, error) {
	sig, err := solana.SignatureFromBase58(signature)
	if err != nil {
		return false, fmt.Errorf("parse signature: %w", err)
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return false, fmt.Errorf("timeout waiting for confirmation")
		}

		<-ticker.C

		status, err := c.rpcClient.GetSignatureStatuses(
			context.Background(),
			true,
			sig,
		)
		if err != nil {
			continue
		}

		if status != nil && len(status.Value) > 0 {
			st := status.Value[0]
			if st != nil {
				if st.Err != nil {
					return false, fmt.Errorf("transaction failed: %v", st.Err)
				}
				if st.ConfirmationStatus == rpc.ConfirmationStatusFinalized {
					return true, nil
				}
			}
		}
	}
}

func (c *Client) GetRecentBlockhash() (solana.Hash, error) {
	result, err := c.rpcClient.GetLatestBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return solana.Hash{}, fmt.Errorf("get recent blockhash: %w", err)
	}
	return result.Value.Blockhash, nil
}

func (c *Client) GetTokenBalance(tokenAccount solana.PublicKey) (uint64, error) {
	result, err := c.rpcClient.GetTokenAccountBalance(context.Background(), tokenAccount, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("get token balance: %w", err)
	}
	amount, err := strconv.ParseUint(result.Value.Amount, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse token amount: %w", err)
	}
	return amount, nil
}

func (c *Client) GetAccountInfo(pubkey solana.PublicKey) (*rpc.GetAccountInfoResult, error) {
	return c.rpcClient.GetAccountInfo(context.Background(), pubkey)
}

func (c *Client) GetProgramAccounts(programID solana.PublicKey) (rpc.GetProgramAccountsResult, error) {
	return c.rpcClient.GetProgramAccounts(context.Background(), programID)
}
