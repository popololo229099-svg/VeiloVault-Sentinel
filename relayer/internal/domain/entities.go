// Package domain contains the core business entities and repository interfaces.
package domain

import (
	"time"

	"github.com/gagliardetto/solana-go"
)

// Transaction represents a relay transaction.
type Transaction struct {
	ID          string           `json:"id" db:"id"`
	Signature   string           `json:"signature" db:"signature"`
	Type        TransactionType  `json:"type" db:"type"`
	Status      TransactionStatus `json:"status" db:"status"`
	Pool        solana.PublicKey `json:"pool" db:"pool"`
	From        solana.PublicKey `json:"from" db:"from"`
	To          solana.PublicKey `json:"to" db:"to"`
	Amount      uint64          `json:"amount" db:"amount"`
	Fee         uint64          `json:"fee" db:"fee"`
	Slot        uint64          `json:"slot" db:"slot"`
	Error       string          `json:"error,omitempty" db:"error"`
	CreatedAt   time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time       `json:"updatedAt" db:"updated_at"`
	ConfirmedAt *time.Time      `json:"confirmedAt,omitempty" db:"confirmed_at"`
}

// TransactionType represents the type of transaction.
type TransactionType string

const (
	TxTypeSwap           TransactionType = "swap"
	TxTypeDeposit        TransactionType = "deposit"
	TxTypeWithdraw       TransactionType = "withdraw"
	TxTypePositionOpen   TransactionType = "position_open"
	TxTypePositionClose  TransactionType = "position_close"
	TxTypePerpReissue    TransactionType = "perp_reissue"
	TxTypePerpRecover    TransactionType = "perp_recover"
)

// TransactionStatus represents the status of a transaction.
type TransactionStatus string

const (
	TxStatusPending    TransactionStatus = "pending"
	TxStatusSubmitted  TransactionStatus = "submitted"
	TxStatusConfirmed  TransactionStatus = "confirmed"
	TxStatusFinalized  TransactionStatus = "finalized"
	TxStatusFailed     TransactionStatus = "failed"
)

// Pool represents a privacy pool.
type Pool struct {
	ID              string           `json:"id" db:"id"`
	Address         solana.PublicKey `json:"address" db:"address"`
	Mint            solana.PublicKey `json:"mint" db:"mint"`
	Vault           solana.PublicKey `json:"vault" db:"vault"`
	Authority       solana.PublicKey `json:"authority" db:"authority"`
	FeeBPS          uint16          `json:"feeBps" db:"fee_bps"`
	MinWithdrawalFee uint64         `json:"minWithdrawalFee" db:"min_withdrawal_fee"`
	IsNativeSOL     bool            `json:"isNativeSol" db:"is_native_sol"`
	IsActive        bool            `json:"isActive" db:"is_active"`
	Balance         uint64          `json:"balance" db:"balance"`
	TotalDeposits   uint64          `json:"totalDeposits" db:"total_deposits"`
	TotalWithdrawals uint64        `json:"totalWithdrawals" db:"total_withdrawals"`
	CreatedAt       time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time       `json:"updatedAt" db:"updated_at"`
}

// Relayer represents a relayer instance.
type Relayer struct {
	ID              string           `json:"id" db:"id"`
	PublicKey       solana.PublicKey `json:"publicKey" db:"public_key"`
	FeeAccount      solana.PublicKey `json:"feeAccount" db:"fee_account"`
	IsActive        bool            `json:"isActive" db:"is_active"`
	TotalRelayed    uint64          `json:"totalRelayed" db:"total_relayed"`
	TotalFeesEarned uint64          `json:"totalFeesEarned" db:"total_fees_earned"`
	TotalErrors     uint64          `json:"totalErrors" db:"total_errors"`
	CreatedAt       time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time       `json:"updatedAt" db:"updated_at"`
}

// RelayerStats represents relayer statistics.
type RelayerStats struct {
	TotalTransactions   int64   `json:"totalTransactions"`
	SuccessfulTxs       int64   `json:"successfulTxs"`
	FailedTxs           int64   `json:"failedTxs"`
	TotalVolume         uint64  `json:"totalVolume"`
	TotalFees           uint64  `json:"totalFees"`
	AverageFee          float64 `json:"averageFee"`
	SuccessRate         float64 `json:"successRate"`
	TxsLast24h          int64   `json:"txsLast24h"`
	VolumeLast24h       uint64  `json:"volumeLast24h"`
	AverageConfirmation float64 `json:"averageConfirmation"`
}

// HealthStatus represents the system health.
type HealthStatus struct {
	Status      string           `json:"status"`
	SolBalance  uint64           `json:"solBalance"`
	SolPrice    float64          `json:"solPrice"`
	Slot        uint64           `json:"slot"`
	Version     string           `json:"version"`
	Uptime      int64            `json:"uptime"`
	Pools       []PoolHealth     `json:"pools"`
}

// PoolHealth represents the health of a pool.
type PoolHealth struct {
	Mint        solana.PublicKey `json:"mint"`
	Balance     uint64          `json:"balance"`
	IsActive    bool            `json:"isActive"`
	LastDeposit *time.Time      `json:"lastDeposit,omitempty"`
}

// TransactionRepository defines the interface for transaction persistence.
type TransactionRepository interface {
	Create(tx *Transaction) error
	Update(tx *Transaction) error
	FindByID(id string) (*Transaction, error)
	FindBySignature(signature string) (*Transaction, error)
	FindByStatus(status TransactionStatus, limit int) ([]*Transaction, error)
	FindByPool(pool solana.PublicKey, limit int) ([]*Transaction, error)
	GetStats(since time.Time) (*RelayerStats, error)
	GetRecentTransactions(limit int) ([]*Transaction, error)
}

// PoolRepository defines the interface for pool persistence.
type PoolRepository interface {
	Create(pool *Pool) error
	Update(pool *Pool) error
	FindByAddress(address solana.PublicKey) (*Pool, error)
	FindByMint(mint solana.PublicKey) (*Pool, error)
	GetActivePools() ([]*Pool, error)
	GetAll() ([]*Pool, error)
}

// RelayerRepository defines the interface for relayer persistence.
type RelayerRepository interface {
	Create(relayer *Relayer) error
	Update(relayer *Relayer) error
	FindByPublicKey(publicKey solana.PublicKey) (*Relayer, error)
	GetActive() (*Relayer, error)
}

// CacheRepository defines the interface for caching.
type CacheRepository interface {
	Set(key string, value interface{}, expiration time.Duration) error
	Get(key string) (string, error)
	Delete(key string) error
	Exists(key string) (bool, error)
}

// SolanaClient defines the interface for Solana interactions.
type SolanaClient interface {
	GetBalance(pubkey solana.PublicKey) (uint64, error)
	GetSlot() (uint64, error)
	SubmitTransaction(tx []byte) (string, error)
	GetTransaction(signature string) (map[string]interface{}, error)
	WaitForConfirmation(signature string, timeout time.Duration) (bool, error)
}

// EventBus defines the interface for event publishing.
type EventBus interface {
	Publish(topic string, message interface{}) error
	Subscribe(topic string, handler func(interface{})) error
}
