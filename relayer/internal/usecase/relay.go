// Package usecase contains the business logic layer.
package usecase

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/rs/zerolog"
	"github.com/popolo229099-svg/veilo-relayer/internal/domain"
)

// RelayUseCase handles transaction relay business logic.
type RelayUseCase struct {
	txRepo    domain.TransactionRepository
	poolRepo  domain.PoolRepository
	relayer   domain.RelayerRepository
	cache     domain.CacheRepository
	solana    domain.SolanaClient
	events    domain.EventBus
	keypair   *solana.Wallet
	logger    zerolog.Logger
	feeBPS    uint16
	minFee    uint64
}

// NewRelayUseCase creates a new relay use case.
func NewRelayUseCase(
	txRepo domain.TransactionRepository,
	poolRepo domain.PoolRepository,
	relayer domain.RelayerRepository,
	cache domain.CacheRepository,
	solana domain.SolanaClient,
	events domain.EventBus,
	keypair *solana.Wallet,
	logger zerolog.Logger,
	feeBPS uint16,
	minFee uint64,
) *RelayUseCase {
	return &RelayUseCase{
		txRepo:   txRepo,
		poolRepo: poolRepo,
		relayer:  relayer,
		cache:    cache,
		solana:   solana,
		events:   events,
		keypair:  keypair,
		logger:   logger,
		feeBPS:   feeBPS,
		minFee:   minFee,
	}
}

// RelayRequest represents a request to relay a transaction.
type RelayRequest struct {
	Type        domain.TransactionType `json:"type"`
	Pool        solana.PublicKey       `json:"pool"`
	Proof       []byte                 `json:"proof"`
	PublicInputs [][]byte              `json:"publicInputs"`
	ExtData     *ExtData               `json:"extData,omitempty"`
	SwapParams  *SwapParams            `json:"swapParams,omitempty"`
	Signers     []solana.PublicKey     `json:"signers"`
}

// ExtData represents extended data for a transaction.
type ExtData struct {
	Fee       uint64          `json:"fee"`
	Relayer   solana.PublicKey `json:"relayer"`
	Recipient solana.PublicKey `json:"recipient"`
}

// SwapParams represents swap parameters.
type SwapParams struct {
	AmountIn     uint64          `json:"amountIn"`
	MinAmountOut uint64          `json:"minAmountOut"`
	Fee          uint64          `json:"fee"`
	Recipient    solana.PublicKey `json:"recipient"`
	Relayer      solana.PublicKey `json:"relayer"`
	SwapDataHash [32]byte        `json:"swapDataHash"`
}

// RelayResponse represents the response from relaying a transaction.
type RelayResponse struct {
	TransactionID string `json:"transactionId"`
	Signature     string `json:"signature"`
	Status        string `json:"status"`
	Fee           uint64 `json:"fee"`
}

// Relay processes a transaction relay request.
func (uc *RelayUseCase) Relay(ctx context.Context, req *RelayRequest) (*RelayResponse, error) {
	uc.logger.Info().
		Str("type", string(req.Type)).
		Str("pool", req.Pool.String()).
		Msg("processing relay request")

	// Validate request
	if err := uc.validateRequest(req); err != nil {
		return nil, fmt.Errorf("validate request: %w", err)
	}

	// Check pool exists and is active
	pool, err := uc.poolRepo.FindByAddress(req.Pool)
	if err != nil {
		return nil, fmt.Errorf("find pool: %w", err)
	}
	if pool == nil || !pool.IsActive {
		return nil, fmt.Errorf("pool not found or inactive")
	}

	// Calculate fee
	fee := uc.calculateFee(req)
	uc.logger.Info().Uint64("fee", fee).Msg("calculated fee")

	// Create transaction record
	txID := generateTxID()
	tx := &domain.Transaction{
		ID:        txID,
		Type:      req.Type,
		Status:    domain.TxStatusPending,
		Pool:      req.Pool,
		Fee:       fee,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := uc.txRepo.Create(tx); err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	// Build and sign transaction
	solanaTx, err := uc.buildTransaction(ctx, req, fee)
	if err != nil {
		tx.Status = domain.TxStatusFailed
		tx.Error = err.Error()
		uc.txRepo.Update(tx)
		return nil, fmt.Errorf("build transaction: %w", err)
	}

	// Submit transaction
	signature, err := uc.solana.SubmitTransaction(solanaTx)
	if err != nil {
		tx.Status = domain.TxStatusFailed
		tx.Error = err.Error()
		uc.txRepo.Update(tx)
		return nil, fmt.Errorf("submit transaction: %w", err)
	}

	tx.Signature = signature
	tx.Status = domain.TxStatusSubmitted
	uc.txRepo.Update(tx)

	// Publish event
	uc.events.Publish("transaction.submitted", tx)

	// Wait for confirmation in background
	go uc.waitForConfirmation(tx)

	return &RelayResponse{
		TransactionID: txID,
		Signature:     signature,
		Status:        string(domain.TxStatusSubmitted),
		Fee:           fee,
	}, nil
}

// validateRequest validates a relay request.
func (uc *RelayUseCase) validateRequest(req *RelayRequest) error {
	if len(req.Proof) == 0 {
		return fmt.Errorf("proof is required")
	}
	if len(req.PublicInputs) == 0 {
		return fmt.Errorf("public inputs are required")
	}
	if req.Type == "" {
		return fmt.Errorf("transaction type is required")
	}
	return nil
}

// calculateFee calculates the relay fee.
func (uc *RelayUseCase) calculateFee(req *RelayRequest) uint64 {
	if req.ExtData != nil && req.ExtData.Fee > 0 {
		return req.ExtData.Fee
	}
	if req.SwapParams != nil && req.SwapParams.Fee > 0 {
		return req.SwapParams.Fee
	}
	return uc.minFee
}

// buildTransaction builds a Solana transaction.
func (uc *RelayUseCase) buildTransaction(ctx context.Context, req *RelayRequest, fee uint64) ([]byte, error) {
	// Get recent blockhash
	recentBlockhash, err := uc.solana.GetSlot()
	if err != nil {
		return nil, fmt.Errorf("get recent blockhash: %w", err)
	}

	_ = recentBlockhash

	// Build instruction based on type
	var instruction *Instruction
	switch req.Type {
	case domain.TxTypeSwap:
		instruction, err = uc.buildSwapInstruction(req, fee)
	case domain.TxTypeDeposit:
		instruction, err = uc.buildDepositInstruction(req, fee)
	case domain.TxTypeWithdraw:
		instruction, err = uc.buildWithdrawInstruction(req, fee)
	default:
		return nil, fmt.Errorf("unsupported transaction type: %s", req.Type)
	}

	if err != nil {
		return nil, err
	}

	// Create and sign transaction
	_ = instruction

	// Simplified - use solana-go Transaction in production
	return []byte{}, nil
}

// buildSwapInstruction builds a swap instruction.
func (uc *RelayUseCase) buildSwapInstruction(req *RelayRequest, fee uint64) (*Instruction, error) {
	if req.SwapParams == nil {
		return nil, fmt.Errorf("swap params required")
	}

	// Build instruction data
	data := make([]byte, 0)
	data = append(data, []byte{0xf8, 0xc6, 0x9e, 0x91, 0xe1, 0x75, 0x87, 0xc8}...) // discriminator

	return &Instruction{
		ProgramID: domain.ProgramID,
		Accounts:  []AccountMeta{},
		Data:      data,
	}, nil
}

// buildDepositInstruction builds a deposit instruction.
func (uc *RelayUseCase) buildDepositInstruction(req *RelayRequest, fee uint64) (*Instruction, error) {
	data := make([]byte, 0)
	data = append(data, []byte{0xf2, 0x99, 0x07, 0x62, 0x13, 0x36, 0x05, 0x83}...) // discriminator

	return &Instruction{
		ProgramID: domain.ProgramID,
		Accounts:  []AccountMeta{},
		Data:      data,
	}, nil
}

// buildWithdrawInstruction builds a withdraw instruction.
func (uc *RelayUseCase) buildWithdrawInstruction(req *RelayRequest, fee uint64) (*Instruction, error) {
	data := make([]byte, 0)
	data = append(data, []byte{0xb0, 0x9e, 0x05, 0x08, 0x44, 0x8f, 0xca, 0x35}...) // discriminator

	return &Instruction{
		ProgramID: domain.ProgramID,
		Accounts:  []AccountMeta{},
		Data:      data,
	}, nil
}

// waitForConfirmation waits for transaction confirmation.
func (uc *RelayUseCase) waitForConfirmation(tx *domain.Transaction) {
	confirmed, err := uc.solana.WaitForConfirmation(tx.Signature, 60*time.Second)
	if err != nil {
		uc.logger.Error().Err(err).Str("signature", tx.Signature).Msg("failed to wait for confirmation")
		return
	}

	if confirmed {
		tx.Status = domain.TxStatusConfirmed
		now := time.Now()
		tx.ConfirmedAt = &now
		uc.txRepo.Update(tx)
		uc.events.Publish("transaction.confirmed", tx)
	} else {
		tx.Status = domain.TxStatusFailed
		tx.Error = "confirmation timeout"
		uc.txRepo.Update(tx)
		uc.events.Publish("transaction.failed", tx)
	}
}

// GetTransaction retrieves a transaction by ID.
func (uc *RelayUseCase) GetTransaction(ctx context.Context, id string) (*domain.Transaction, error) {
	return uc.txRepo.FindByID(id)
}

// GetTransactionBySignature retrieves a transaction by signature.
func (uc *RelayUseCase) GetTransactionBySignature(ctx context.Context, signature string) (*domain.Transaction, error) {
	return uc.txRepo.FindBySignature(signature)
}

// GetRecentTransactions retrieves recent transactions.
func (uc *RelayUseCase) GetRecentTransactions(ctx context.Context, limit int) ([]*domain.Transaction, error) {
	return uc.txRepo.GetRecentTransactions(limit)
}

// GetStats retrieves relayer statistics.
func (uc *RelayUseCase) GetStats(ctx context.Context) (*domain.RelayerStats, error) {
	since := time.Now().Add(-24 * time.Hour)
	return uc.txRepo.GetStats(since)
}

// GetHealth retrieves system health.
func (uc *RelayUseCase) GetHealth(ctx context.Context) (*domain.HealthStatus, error) {
	balance, err := uc.solana.GetBalance(uc.keypair.PublicKey())
	if err != nil {
		return nil, err
	}

	slot, err := uc.solana.GetSlot()
	if err != nil {
		return nil, err
	}

	pools, err := uc.poolRepo.GetActivePools()
	if err != nil {
		return nil, err
	}

	poolHealth := make([]domain.PoolHealth, len(pools))
	for i, p := range pools {
		poolHealth[i] = domain.PoolHealth{
			Mint:     p.Mint,
			Balance:  p.Balance,
			IsActive: p.IsActive,
		}
	}

	return &domain.HealthStatus{
		Status:     "healthy",
		SolBalance: balance,
		Slot:       slot,
		Version:    "1.0.0",
		Uptime:     time.Now().Unix(),
		Pools:      poolHealth,
	}, nil
}

// Instruction represents a Solana instruction.
type Instruction struct {
	ProgramID solana.PublicKey
	Accounts  []AccountMeta
	Data      []byte
}

// AccountMeta represents account metadata.
type AccountMeta struct {
	Pubkey     solana.PublicKey
	IsSigner   bool
	IsWritable bool
}

// generateTxID generates a unique transaction ID.
func generateTxID() string {
	return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000000")))
}
