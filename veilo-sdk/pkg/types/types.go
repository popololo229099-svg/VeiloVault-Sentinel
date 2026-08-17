// Package types provides type definitions for the Veilo Privacy Pool SDK.
package types

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// Program IDs
var (
	ProgramID = solana.MustPublicKeyFromBase58("GYy4kM6GHhpgLCUscuABbzkD2ZbJ2fneYryaZ6Ch7fFU")

	// System programs
	TokenProgram        = solana.TokenProgramID
	AssociatedTokenProg = solana.AssociatedTokenProgramID
	SystemProgram       = solana.SystemProgramID
	RentProgram         = solana.SysvarRent111111111111111111111111111111111
	InstructionsSysvar  = solana.SysvarInstructions1111111111111111111111111
)

// Severity represents a finding severity level.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityLow
	SeverityMedium
	SeverityHigh
)

func (s Severity) String() string {
	switch s {
	case SeverityHigh:
		return "HIGH"
	case SeverityMedium:
		return "MEDIUM"
	case SeverityLow:
		return "LOW"
	default:
		return "INFO"
	}
}

// PoolConfig represents the on-chain pool configuration.
type PoolConfig struct {
	Authority          solana.PublicKey `json:"authority"`
	Mint               solana.PublicKey `json:"mint"`
	Vault              solana.PublicKey `json:"vault"`
	FeeBPS             uint16          `json:"feeBps"`
	MinWithdrawalFee   uint64          `json:"minWithdrawalFee"`
	FeeErrorMarginBPS  uint16          `json:"feeErrorMarginBps"`
	IsNativeSOL        bool            `json:"isNativeSol"`
	IsInitialized      bool            `json:"isInitialized"`
	Nonce              uint8           `json:"nonce"`
}

// SwapParams represents the parameters for a Jupiter swap transaction.
type SwapParams struct {
	AmountIn       uint64          `json:"amountIn"`
	MinAmountOut   uint64          `json:"minAmountOut"`
	Fee            uint64          `json:"fee"`
	Recipient      solana.PublicKey `json:"recipient"`
	Relayer        solana.PublicKey `json:"relayer"`
	SwapDataHash   [32]byte        `json:"swapDataHash"`
}

// Hash computes the Poseidon hash of the swap parameters (public input for ZK proof).
func (sp *SwapParams) Hash() [32]byte {
	// Simplified hash - actual implementation uses Poseidon
	data := make([]byte, 0, 32+32+8+8+8+32)
	data = append(data, sp.Recipient.Bytes()...)
	data = append(data, sp.Relayer.Bytes()...)

	amountBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, sp.AmountIn)
	data = append(data, amountBytes...)

	minBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(minBytes, sp.MinAmountOut)
	data = append(data, minBytes...)

	feeBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(feeBytes, sp.Fee)
	data = append(data, feeBytes...)

	// SHA256 as placeholder for Poseidon
	return sha256Hash(data)
}

// ExtData represents extended data for a transaction.
type ExtData struct {
	Fee       uint64          `json:"fee"`
	Relayer   solana.PublicKey `json:"relayer"`
	Recipient solana.PublicKey `json:"recipient"`
}

// Nullifier represents a spent note nullifier.
type Nullifier struct {
	IsNullified bool   `json:"isNullified"`
	Slot        uint64 `json:"slot"`
}

// PositionPool represents a cross-mint position pool.
type PositionPool struct {
	Authority    solana.PublicKey `json:"authority"`
	MintA        solana.PublicKey `json:"mintA"`
	MintB        solana.PublicKey `json:"mintB"`
	PoolA        solana.PublicKey `json:"poolA"`
	PoolB        solana.PublicKey `json:"poolB"`
	IsOpen       bool            `json:"isOpen"`
	Nonce        uint8           `json:"nonce"`
}

// PhoenixSlot represents a Phoenix Eternal perpetual slot.
type PhoenixSlot struct {
	Amount        uint64 `json:"amount"`
	Reissued      uint64 `json:"reissued"`
	MaxSlotAmount uint64 `json:"maxSlotAmount"`
	IsInitialized bool   `json:"isInitialized"`
}

// JupiterPerpParams represents Jupiter Perpetuals parameters.
type JupiterPerpParams struct {
	MarketIndex   uint16          `json:"marketIndex"`
	Amount        uint64          `json:"amount"`
	Direction     uint8           `json:"direction"`
	Collateral    uint64          `json:"collateral"`
	Leverage      uint16          `json:"leverage"`
	Relayer       solana.PublicKey `json:"relayer"`
	Recipient     solana.PublicKey `json:"recipient"`
	Fee           uint64          `json:"fee"`
}

// TransactionRequest represents a request to submit a transaction.
type TransactionRequest struct {
	Type        string            `json:"type"` // "swap", "deposit", "withdraw", "position_open", "position_close", "perp_reissue", "perp_recover"
	Pool        solana.PublicKey  `json:"pool"`
	Proof       []byte            `json:"proof"`
	PublicInputs [][]byte          `json:"publicInputs"`
	ExtData     *ExtData          `json:"extData,omitempty"`
	SwapParams  *SwapParams       `json:"swapParams,omitempty"`
	Signers     []solana.PublicKey `json:"signers"`
}

// TransactionResponse represents the response after submitting a transaction.
type TransactionResponse struct {
	Signature string `json:"signature"`
	Status    string `json:"status"`
	Slot      uint64 `json:"slot,omitempty"`
	Error     string `json:"error,omitempty"`
}

// RelayStatus represents the status of a relay.
type RelayStatus struct {
	Signature string `json:"signature"`
	Status    string `json:"status"` // "pending", "confirmed", "finalized", "failed"
	Slot      uint64 `json:"slot"`
	Fee       uint64 `json:"fee"`
	Timestamp int64 `json:"timestamp"`
}

// HealthStatus represents the relayer health.
type HealthStatus struct {
	Status      string `json:"status"`
	SolBalance  uint64 `json:"solBalance"`
	Slot        uint64 `json:"slot"`
	Version     string `json:"version"`
	Uptime      int64  `json:"uptime"`
}

// Helper functions
func sha256Hash(data []byte) [32]byte {
	// Simplified - use crypto/sha256 in production
	var hash [32]byte
	copy(hash[:], data[:min(len(data), 32)])
	return hash
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// PDA helpers
func FindPoolConfigPDA(mint solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{[]byte("pool_config"), mint.Bytes()},
		ProgramID,
	)
}

func FindVaultPDA(mint solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{[]byte("vault"), mint.Bytes()},
		ProgramID,
	)
}

func FindNullifierPDA(nullifier [32]byte) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{[]byte("nullifier"), nullifier[:]},
		ProgramID,
	)
}

func FindPositionPoolPDA(mintA, mintB solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{[]byte("position_pool"), mintA.Bytes(), mintB.Bytes()},
		ProgramID,
	)
}

func FindPhoenixSlotPDA(pool, user solana.PublicKey) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{[]byte("phoenix_slot"), pool.Bytes(), user.Bytes()},
		ProgramID,
	)
}

// AccountInfo represents parsed account info.
type AccountInfo struct {
	Lamports   uint64        `json:"lamports"`
	Owner      solana.PublicKey `json:"owner"`
	Data       []byte        `json:"data"`
	Executable bool          `json:"executable"`
	RentEpoch  uint64        `json:"rentEpoch"`
}

// RPCClient wraps the Solana RPC client.
type RPCClient struct {
	*rpc.Client
}

// NewRPCClient creates a new RPC client.
func NewRPCClient(endpoint string) *RPCClient {
	return &RPCClient{
		Client: rpc.New(endpoint),
	}
}

// GetBalance returns the SOL balance for an account.
func (c *RPCClient) GetBalance(pubkey solana.PublicKey) (uint64, error) {
	result, err := c.Client.GetBalance(nil, pubkey, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}
	return result.Value, nil
}

// GetTokenBalance returns the token balance for an account.
func (c *RPCClient) GetTokenBalance(tokenAccount solana.PublicKey) (uint64, error) {
	result, err := c.Client.GetTokenAccountBalance(nil, tokenAccount, rpc.CommitmentFinalized)
	if err != nil {
		return 0, err
	}
	return result.Value.AmountUint64, nil
}

// Errors
var (
	ErrInvalidProof       = errors.New("invalid zero-knowledge proof")
	ErrInsufficientFunds  = errors.New("insufficient funds")
	ErrPoolNotFound       = errors.New("pool not found")
	ErrNullifierSpent     = errors.New("nullifier already spent")
	ErrInvalidMint        = errors.New("invalid mint address")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrRelayerUnavailable = errors.New("relayer unavailable")
)

// big.Int placeholder for BN254 field operations
var BN254Field = func() *big.Int {
	// BN254 field order
	field, _ := new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)
	return field
}()
