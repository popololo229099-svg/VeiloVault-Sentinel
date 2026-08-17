// Package client provides the Veilo Privacy Pool SDK client.
package client

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/popolo229099-svg/veilo-sdk/pkg/types"
)

// Client is the main Veilo SDK client.
type Client struct {
	rpcClient   *types.RPCClient
	wsClient    *ws.Client
	programID   solana.PublicKey
	commitment  rpc.CommitmentType
}

// Config represents the client configuration.
type Config struct {
	RPCEndpoint  string
	WSEndpoint   string
	ProgramID    solana.PublicKey
	Commitment   rpc.CommitmentType
}

// DefaultConfig returns default client configuration.
func DefaultConfig() Config {
	return Config{
		RPCEndpoint: "https://api.mainnet-beta.solana.com",
		WSEndpoint:  "wss://api.mainnet-beta.solana.com",
		ProgramID:   types.ProgramID,
		Commitment:  rpc.CommitmentFinalized,
	}
}

// NewClient creates a new Veilo SDK client.
func NewClient(config Config) *Client {
	rpcClient := types.NewRPCClient(config.RPCEndpoint)
	wsClient := ws.New(config.WSEndpoint)

	return &Client{
		rpcClient:  rpcClient,
		wsClient:   wsClient,
		programID:  config.ProgramID,
		commitment: config.Commitment,
	}
}

// GetPoolConfig fetches the pool configuration from on-chain.
func (c *Client) GetPoolConfig(ctx context.Context, mint solana.PublicKey) (*types.PoolConfig, error) {
	pda, _, err := types.FindPoolConfigPDA(mint)
	if err != nil {
		return nil, fmt.Errorf("find pool config PDA: %w", err)
	}

	result, err := c.rpcClient.GetAccountInfo(ctx, pda)
	if err != nil {
		return nil, fmt.Errorf("get account info: %w", err)
	}

	if result == nil || result.Value == nil {
		return nil, types.ErrPoolNotFound
	}

	config := &types.PoolConfig{}
	if err := config.Deserialize(result.Value.Data.GetBinary()); err != nil {
		return nil, fmt.Errorf("deserialize pool config: %w", err)
	}

	return config, nil
}

// GetVaultBalance fetches the vault balance for a pool.
func (c *Client) GetVaultBalance(ctx context.Context, mint solana.PublicKey) (uint64, error) {
	vault, _, err := types.FindVaultPDA(mint)
	if err != nil {
		return 0, fmt.Errorf("find vault PDA: %w", err)
	}

	if mint.Equals(solana.NativeMint) {
		return c.rpcClient.GetBalance(vault)
	}

	return c.rpcClient.GetTokenBalance(vault)
}

// IsNullifierSpent checks if a nullifier has been spent.
func (c *Client) IsNullifierSpent(ctx context.Context, nullifier [32]byte) (bool, error) {
	pda, _, err := types.FindNullifierPDA(nullifier)
	if err != nil {
		return false, fmt.Errorf("find nullifier PDA: %w", err)
	}

	result, err := c.rpcClient.GetAccountInfo(ctx, pda)
	if err != nil {
		return false, fmt.Errorf("get account info: %w", err)
	}

	if result == nil || result.Value == nil {
		return false, nil
	}

	return true, nil
}

// GetPoolStatus returns the current status of a pool.
func (c *Client) GetPoolStatus(ctx context.Context, mint solana.PublicKey) (*PoolStatus, error) {
	config, err := c.GetPoolConfig(ctx, mint)
	if err != nil {
		return nil, err
	}

	balance, err := c.GetVaultBalance(ctx, mint)
	if err != nil {
		return nil, err
	}

	return &PoolStatus{
		Config:    config,
		Balance:   balance,
		IsHealthy: config.IsInitialized && balance > 0,
	}, nil
}

// GetRecentSlot returns the current slot.
func (c *Client) GetRecentSlot(ctx context.Context) (uint64, error) {
	slot, err := c.rpcClient.GetSlot(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}
	return slot, nil
}

// GetTransaction fetches a transaction by signature.
func (c *Client) GetTransaction(ctx context.Context, sig solana.Signature) (*types.TransactionResponse, error) {
	result, err := c.rpcClient.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{
		Encoding:   "jsonParsed",
		Commitment: rpc.CommitmentFinalized,
	})
	if err != nil {
		return nil, err
	}

	if result == nil {
		return &types.TransactionResponse{
			Signature: sig.String(),
			Status:    "not_found",
		}, nil
	}

	status := "confirmed"
	if result.Meta != nil && result.Meta.Err != nil {
		status = "failed"
	}

	return &types.TransactionResponse{
		Signature: sig.String(),
		Status:    status,
		Slot:      result.Slot,
	}, nil
}

// SubscribeSlot subscribes to new slot notifications.
func (c *Client) SubscribeSlot(ctx context.Context) (*ws.SlotsSub, error) {
	return c.wsClient.SlotSubscribe(ctx, rpc.CommitmentFinalized)
}

// PoolStatus represents the current pool status.
type PoolStatus struct {
	Config    *types.PoolConfig `json:"config"`
	Balance   uint64            `json:"balance"`
	IsHealthy bool              `json:"isHealthy"`
}

// Deserialize deserializes the pool config from bytes.
func (pc *types.PoolConfig) Deserialize(data []byte) error {
	if len(data) < 32+32+32+2+8+2+1+1+1 {
		return fmt.Errorf("insufficient data for pool config")
	}

	offset := 0
	copy(pc.Authority[:], data[offset:offset+32])
	offset += 32
	copy(pc.Mint[:], data[offset:offset+32])
	offset += 32
	copy(pc.Vault[:], data[offset:offset+32])
	offset += 32
	pc.FeeBPS = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	pc.MinWithdrawalFee = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	pc.FeeErrorMarginBPS = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	pc.IsNativeSOL = data[offset] != 0
	offset += 1
	pc.IsInitialized = data[offset] != 0
	offset += 1
	pc.Nonce = data[offset]

	return nil
}

// Close closes the client connections.
func (c *Client) Close() {
	if c.wsClient != nil {
		c.wsClient.Close()
	}
}

// Health returns the health status of the relayer.
func (c *Client) Health(ctx context.Context) (*types.HealthStatus, error) {
	slot, err := c.GetRecentSlot(ctx)
	if err != nil {
		return nil, err
	}

	return &types.HealthStatus{
		Status:  "healthy",
		Slot:    slot,
		Version: "1.0.0",
		Uptime:  time.Now().Unix(),
	}, nil
}
