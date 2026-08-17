// Package solana provides Solana blockchain client implementation.
package solana

import (
	"context"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/rs/zerolog"
)

// Client implements the SolanaClient interface.
type Client struct {
	rpcClient   *rpc.Client
	wsEndpoint  string
	logger      zerolog.Logger
}

// NewClient creates a new Solana client.
func NewClient(rpcEndpoint, wsEndpoint string, logger zerolog.Logger) *Client {
	return &Client{
		rpcClient:  rpc.New(rpcEndpoint),
		wsEndpoint: wsEndpoint,
		logger:     logger,
	}
}

// GetBalance returns the SOL balance for an account.
func (c *Client) GetBalance(pubkey solana.PublicKey) (uint64, error) {
	result, err := c.rpcClient.GetBalance(nil, pubkey, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("get balance: %w", err)
	}
	return result.Value, nil
}

// GetSlot returns the current slot.
func (c *Client) GetSlot() (uint64, error) {
	slot, err := c.rpcClient.GetSlot(nil, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("get slot: %w", err)
	}
	return slot, nil
}

// SubmitTransaction submits a signed transaction.
func (c *Client) SubmitTransaction(txBytes []byte) (string, error) {
	sig, err := c.rpcClient.SendTransaction(
		nil,
		txBytes,
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return "", fmt.Errorf("send transaction: %w", err)
	}
	return sig.String(), nil
}

// GetTransaction fetches a transaction by signature.
func (c *Client) GetTransaction(signature string) (map[string]interface{}, error) {
	sig, err := solana.SignatureFromBase58(signature)
	if err != nil {
		return nil, fmt.Errorf("parse signature: %w", err)
	}

	result, err := c.rpcClient.GetTransaction(
		nil,
		sig,
		&rpc.GetTransactionOpts{
			Encoding:   "jsonParsed",
			Commitment: rpc.CommitmentFinalized,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	// Convert result to map
	return map[string]interface{}{
		"signature": signature,
		"slot":      result.Slot,
	}, nil
}

// WaitForConfirmation waits for transaction confirmation.
func (c *Client) WaitForConfirmation(signature string, timeout time.Duration) (bool, error) {
	sig, err := solana.SignatureFromBase58(signature)
	if err != nil {
		return false, fmt.Errorf("parse signature: %w", err)
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-time.After(time.Until(deadline)):
			return false, fmt.Errorf("timeout waiting for confirmation")
		case <-ticker.C:
			status, err := c.rpcClient.GetSignatureStatuses(
				nil,
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
}

// GetRecentBlockhash returns a recent blockhash.
func (c *Client) GetRecentBlockhash() (solana.Hash, error) {
	result, err := c.rpcClient.GetRecentBlockhash(nil, rpc.CommitmentFinalized)
	if err != nil {
		return solana.Hash{}, fmt.Errorf("get recent blockhash: %w", err)
	}
	return result.Value.Blockhash, nil
}

// GetTokenBalance returns the token balance for an account.
func (c *Client) GetTokenBalance(tokenAccount solana.PublicKey) (uint64, error) {
	result, err := c.rpcClient.GetTokenAccountBalance(nil, tokenAccount, rpc.CommitmentFinalized)
	if err != nil {
		return 0, fmt.Errorf("get token balance: %w", err)
	}
	return result.Value.AmountUint64, nil
}

// GetAccountInfo returns account info.
func (c *Client) GetAccountInfo(pubkey solana.PublicKey) (*rpc.GetAccountInfoResult, error) {
	return c.rpcClient.GetAccountInfo(nil, pubkey)
}

// GetProgramAccounts returns all accounts owned by a program.
func (c *Client) GetProgramAccounts(programID solana.PublicKey) ([]rpc.KeyedAccount, error) {
	return c.rpcClient.GetProgramAccounts(nil, programID, nil)
}

// SimulateTransaction simulates a transaction.
func (c *Client) SimulateTransaction(txBytes []byte) (*rpc.SimulateTransactionResult, error) {
	return c.rpcClient.SimulateTransaction(nil, txBytes, nil)
}
