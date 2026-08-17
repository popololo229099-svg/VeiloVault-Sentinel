// Package factory implements the Factory Method pattern for transaction builders.
// Each transaction type has its own concrete builder that implements TransactionBuilder.
// The factory registers builders by TransactionType and creates the appropriate one.
//
// Ref: https://refactoring.guru/design-patterns/factory-method
package factory

import (
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/popolo229099-svg/veilo-relayer/internal/domain"
)

// TransactionBuilder defines the interface that all concrete builders implement.
// This is the Product interface in the Factory Method pattern.
type TransactionBuilder interface {
	Build() (*domain.Transaction, error)
	Type() domain.TransactionType
}

// TransactionBuilderFactory is the Creator in the Factory Method pattern.
// It holds a registry of builder constructors keyed by TransactionType.
type TransactionBuilderFactory struct {
	registry map[domain.TransactionType]BuilderConstructor
}

// BuilderConstructor is a function that creates a TransactionBuilder from config.
type BuilderConstructor func(cfg BuilderConfig) TransactionBuilder

// BuilderConfig holds common configuration passed to all builders.
type BuilderConfig struct {
	Pool        solana.PublicKey
	Proof       []byte
	PublicInputs [][]byte
	Fee         uint64
	RelayerKey  solana.PublicKey
}

// NewTransactionBuilderFactory creates a factory with all registered builders.
func NewTransactionBuilderFactory() *TransactionBuilderFactory {
	f := &TransactionBuilderFactory{
		registry: make(map[domain.TransactionType]BuilderConstructor),
	}

	// Register default builders
	f.Register(domain.TxTypeSwap, NewSwapBuilder)
	f.Register(domain.TxTypeDeposit, NewDepositBuilder)
	f.Register(domain.TxTypeWithdraw, NewWithdrawBuilder)
	f.Register(domain.TxTypePositionOpen, NewPositionOpenBuilder)
	f.Register(domain.TxTypePositionClose, NewPositionCloseBuilder)
	f.Register(domain.TxTypePerpReissue, NewPerpReissueBuilder)
	f.Register(domain.TxTypePerpRecover, NewPerpRecoverBuilder)

	return f
}

// Register adds a builder constructor for a given transaction type.
func (f *TransactionBuilderFactory) Register(txType domain.TransactionType, constructor BuilderConstructor) {
	f.registry[txType] = constructor
}

// Create is the Factory Method — it returns a concrete builder for the requested type.
func (f *TransactionBuilderFactory) Create(txType domain.TransactionType, cfg BuilderConfig) (TransactionBuilder, error) {
	constructor, ok := f.registry[txType]
	if !ok {
		return nil, fmt.Errorf("unsupported transaction type: %s", txType)
	}
	return constructor(cfg), nil
}

// SupportedTypes returns all registered transaction types.
func (f *TransactionBuilderFactory) SupportedTypes() []domain.TransactionType {
	types := make([]domain.TransactionType, 0, len(f.registry))
	for t := range f.registry {
		types = append(types, t)
	}
	return types
}

// --- Concrete Builders ---

type baseBuilder struct {
	cfg BuilderConfig
}

func (b *baseBuilder) baseFields() *domain.Transaction {
	return &domain.Transaction{
		Pool:        b.cfg.Pool,
		Fee:         b.cfg.Fee,
		Amount:      0,
		From:        b.cfg.RelayerKey,
		Status:      domain.TxStatusPending,
	}
}

// SwapBuilder builds swap transactions.
type SwapBuilder struct{ baseBuilder }

func NewSwapBuilder(cfg BuilderConfig) TransactionBuilder {
	return &SwapBuilder{baseBuilder{cfg: cfg}}
}

func (b *SwapBuilder) Type() domain.TransactionType { return domain.TxTypeSwap }

func (b *SwapBuilder) Build() (*domain.Transaction, error) {
	if len(b.cfg.Proof) == 0 {
		return nil, fmt.Errorf("swap requires proof")
	}
	tx := b.baseFields()
	tx.Type = domain.TxTypeSwap
	return tx, nil
}

// DepositBuilder builds deposit transactions.
type DepositBuilder struct{ baseBuilder }

func NewDepositBuilder(cfg BuilderConfig) TransactionBuilder {
	return &DepositBuilder{baseBuilder{cfg: cfg}}
}

func (b *DepositBuilder) Type() domain.TransactionType { return domain.TxTypeDeposit }

func (b *DepositBuilder) Build() (*domain.Transaction, error) {
	if len(b.cfg.Proof) == 0 {
		return nil, fmt.Errorf("deposit requires proof")
	}
	tx := b.baseFields()
	tx.Type = domain.TxTypeDeposit
	return tx, nil
}

// WithdrawBuilder builds withdraw transactions.
type WithdrawBuilder struct{ baseBuilder }

func NewWithdrawBuilder(cfg BuilderConfig) TransactionBuilder {
	return &WithdrawBuilder{baseBuilder{cfg: cfg}}
}

func (b *WithdrawBuilder) Type() domain.TransactionType { return domain.TxTypeWithdraw }

func (b *WithdrawBuilder) Build() (*domain.Transaction, error) {
	if len(b.cfg.Proof) == 0 {
		return nil, fmt.Errorf("withdraw requires proof")
	}
	tx := b.baseFields()
	tx.Type = domain.TxTypeWithdraw
	return tx, nil
}

// PositionOpenBuilder builds position open transactions.
type PositionOpenBuilder struct{ baseBuilder }

func NewPositionOpenBuilder(cfg BuilderConfig) TransactionBuilder {
	return &PositionOpenBuilder{baseBuilder{cfg: cfg}}
}

func (b *PositionOpenBuilder) Type() domain.TransactionType { return domain.TxTypePositionOpen }

func (b *PositionOpenBuilder) Build() (*domain.Transaction, error) {
	tx := b.baseFields()
	tx.Type = domain.TxTypePositionOpen
	return tx, nil
}

// PositionCloseBuilder builds position close transactions.
type PositionCloseBuilder struct{ baseBuilder }

func NewPositionCloseBuilder(cfg BuilderConfig) TransactionBuilder {
	return &PositionCloseBuilder{baseBuilder{cfg: cfg}}
}

func (b *PositionCloseBuilder) Type() domain.TransactionType { return domain.TxTypePositionClose }

func (b *PositionCloseBuilder) Build() (*domain.Transaction, error) {
	tx := b.baseFields()
	tx.Type = domain.TxTypePositionClose
	return tx, nil
}

// PerpReissueBuilder builds perp reissue transactions.
type PerpReissueBuilder struct{ baseBuilder }

func NewPerpReissueBuilder(cfg BuilderConfig) TransactionBuilder {
	return &PerpReissueBuilder{baseBuilder{cfg: cfg}}
}

func (b *PerpReissueBuilder) Type() domain.TransactionType { return domain.TxTypePerpReissue }

func (b *PerpReissueBuilder) Build() (*domain.Transaction, error) {
	if len(b.cfg.Proof) == 0 {
		return nil, fmt.Errorf("perp reissue requires proof")
	}
	tx := b.baseFields()
	tx.Type = domain.TxTypePerpReissue
	return tx, nil
}

// PerpRecoverBuilder builds perp recover transactions.
type PerpRecoverBuilder struct{ baseBuilder }

func NewPerpRecoverBuilder(cfg BuilderConfig) TransactionBuilder {
	return &PerpRecoverBuilder{baseBuilder{cfg: cfg}}
}

func (b *PerpRecoverBuilder) Type() domain.TransactionType { return domain.TxTypePerpRecover }

func (b *PerpRecoverBuilder) Build() (*domain.Transaction, error) {
	tx := b.baseFields()
	tx.Type = domain.TxTypePerpRecover
	return tx, nil
}
