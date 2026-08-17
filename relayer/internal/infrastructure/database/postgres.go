// Package database provides PostgreSQL database implementation.
package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/popolo229099-svg/veilo-relayer/internal/domain"
)

// PostgresDB implements the repository interfaces using PostgreSQL.
type PostgresDB struct {
	db *sqlx.DB
}

// NewPostgresDB creates a new PostgreSQL database connection.
func NewPostgresDB(dsn string) (*PostgresDB, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &PostgresDB{db: db}, nil
}

// Migrate runs database migrations.
func (d *PostgresDB) Migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS transactions (
			id VARCHAR(64) PRIMARY KEY,
			signature VARCHAR(128),
			type VARCHAR(32) NOT NULL,
			status VARCHAR(32) NOT NULL,
			pool VARCHAR(64) NOT NULL",
			"from" VARCHAR(64),
			"to" VARCHAR(64),
			amount BIGINT DEFAULT 0,
			fee BIGINT DEFAULT 0,
			slot BIGINT DEFAULT 0,
			error TEXT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			confirmed_at TIMESTAMP WITH TIME ZONE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_pool ON transactions(pool)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_created ON transactions(created_at)`,
		`CREATE TABLE IF NOT EXISTS pools (
			id VARCHAR(64) PRIMARY KEY,
			address VARCHAR(64) UNIQUE NOT NULL,
			mint VARCHAR(64) NOT NULL,
			vault VARCHAR(64) NOT NULL,
			authority VARCHAR(64) NOT NULL,
			fee_bps SMALLINT DEFAULT 0,
			min_withdrawal_fee BIGINT DEFAULT 0,
			is_native_sol BOOLEAN DEFAULT FALSE,
			is_active BOOLEAN DEFAULT TRUE,
			balance BIGINT DEFAULT 0,
			total_deposits BIGINT DEFAULT 0,
			total_withdrawals BIGINT DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS relayers (
			id VARCHAR(64) PRIMARY KEY,
			public_key VARCHAR(64) UNIQUE NOT NULL,
			fee_account VARCHAR(64) NOT NULL,
			is_active BOOLEAN DEFAULT TRUE,
			total_relayed BIGINT DEFAULT 0,
			total_fees_earned BIGINT DEFAULT 0,
			total_errors BIGINT DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
	}

	for _, m := range migrations {
		if _, err := d.db.Exec(m); err != nil {
			return fmt.Errorf("run migration: %w", err)
		}
	}

	return nil
}

// Close closes the database connection.
func (d *PostgresDB) Close() error {
	return d.db.Close()
}

// TransactionRepository implements domain.TransactionRepository.
type TransactionRepository struct {
	db *PostgresDB
}

// NewTransactionRepository creates a new transaction repository.
func NewTransactionRepository(db *PostgresDB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Create creates a new transaction.
func (r *TransactionRepository) Create(tx *domain.Transaction) error {
	query := `
		INSERT INTO transactions (id, signature, type, status, pool, "from", "to", amount, fee, slot, error, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.db.db.Exec(query,
		tx.ID, tx.Signature, tx.Type, tx.Status,
		tx.Pool.String(), tx.From.String(), tx.To.String(),
		tx.Amount, tx.Fee, tx.Slot, tx.Error,
		tx.CreatedAt, tx.UpdatedAt,
	)
	return err
}

// Update updates a transaction.
func (r *TransactionRepository) Update(tx *domain.Transaction) error {
	query := `
		UPDATE transactions 
		SET status = $2, slot = $3, error = $4, confirmed_at = $5, updated_at = $6
		WHERE id = $1
	`
	_, err := r.db.db.Exec(query,
		tx.ID, tx.Status, tx.Slot, tx.Error,
		tx.ConfirmedAt, tx.UpdatedAt,
	)
	return err
}

// FindByID finds a transaction by ID.
func (r *TransactionRepository) FindByID(id string) (*domain.Transaction, error) {
	tx := &domain.Transaction{}
	query := `SELECT * FROM transactions WHERE id = $1`
	err := r.db.db.Get(tx, query, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return tx, err
}

// FindBySignature finds a transaction by signature.
func (r *TransactionRepository) FindBySignature(signature string) (*domain.Transaction, error) {
	tx := &domain.Transaction{}
	query := `SELECT * FROM transactions WHERE signature = $1`
	err := r.db.db.Get(tx, query, signature)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return tx, err
}

// FindByStatus finds transactions by status.
func (r *TransactionRepository) FindByStatus(status domain.TransactionStatus, limit int) ([]*domain.Transaction, error) {
	var txs []*domain.Transaction
	query := `SELECT * FROM transactions WHERE status = $1 ORDER BY created_at DESC LIMIT $2`
	err := r.db.db.Select(&txs, query, status, limit)
	return txs, err
}

// FindByPool finds transactions by pool.
func (r *TransactionRepository) FindByPool(pool solana.PublicKey, limit int) ([]*domain.Transaction, error) {
	var txs []*domain.Transaction
	query := `SELECT * FROM transactions WHERE pool = $1 ORDER BY created_at DESC LIMIT $2`
	err := r.db.db.Select(&txs, query, pool.String(), limit)
	return txs, err
}

// GetStats gets relayer statistics.
func (r *TransactionRepository) GetStats(since time.Time) (*domain.RelayerStats, error) {
	stats := &domain.RelayerStats{}

	query := `
		SELECT 
			COUNT(*) as total_transactions,
			COUNT(CASE WHEN status = 'confirmed' OR status = 'finalized' THEN 1 END) as successful_txs,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed_txs,
			COALESCE(SUM(amount), 0) as total_volume,
			COALESCE(SUM(fee), 0) as total_fees,
			COALESCE(AVG(fee), 0) as average_fee,
			COUNT(CASE WHEN created_at >= $1 THEN 1 END) as txs_last_24h,
			COALESCE(SUM(CASE WHEN created_at >= $1 THEN amount ELSE 0 END), 0) as volume_last_24h
		FROM transactions
		WHERE created_at >= $1
	`
	err := r.db.db.Get(stats, query, since)
	if err != nil {
		return nil, err
	}

	// Calculate success rate
	if stats.TotalTransactions > 0 {
		stats.SuccessRate = float64(stats.SuccessfulTxs) / float64(stats.TotalTransactions) * 100
	}

	return stats, nil
}

// GetRecentTransactions gets recent transactions.
func (r *TransactionRepository) GetRecentTransactions(limit int) ([]*domain.Transaction, error) {
	var txs []*domain.Transaction
	query := `SELECT * FROM transactions ORDER BY created_at DESC LIMIT $1`
	err := r.db.db.Select(&txs, query, limit)
	return txs, err
}

// PoolRepository implements domain.PoolRepository.
type PoolRepository struct {
	db *PostgresDB
}

// NewPoolRepository creates a new pool repository.
func NewPoolRepository(db *PostgresDB) *PoolRepository {
	return &PoolRepository{db: db}
}

// Create creates a new pool.
func (r *PoolRepository) Create(pool *domain.Pool) error {
	query := `
		INSERT INTO pools (id, address, mint, vault, authority, fee_bps, min_withdrawal_fee, is_native_sol, is_active, balance, total_deposits, total_withdrawals, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := r.db.db.Exec(query,
		pool.ID, pool.Address.String(), pool.Mint.String(), pool.Vault.String(),
		pool.Authority.String(), pool.FeeBPS, pool.MinWithdrawalFee,
		pool.IsNativeSOL, pool.IsActive, pool.Balance,
		pool.TotalDeposits, pool.TotalWithdrawals,
		pool.CreatedAt, pool.UpdatedAt,
	)
	return err
}

// Update updates a pool.
func (r *PoolRepository) Update(pool *domain.Pool) error {
	query := `
		UPDATE pools 
		SET balance = $2, total_deposits = $3, total_withdrawals = $4, is_active = $5, updated_at = $6
		WHERE address = $1
	`
	_, err := r.db.db.Exec(query,
		pool.Address.String(), pool.Balance,
		pool.TotalDeposits, pool.TotalWithdrawals,
		pool.IsActive, pool.UpdatedAt,
	)
	return err
}

// FindByAddress finds a pool by address.
func (r *PoolRepository) FindByAddress(address solana.PublicKey) (*domain.Pool, error) {
	pool := &domain.Pool{}
	query := `SELECT * FROM pools WHERE address = $1`
	err := r.db.db.Get(pool, query, address.String())
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return pool, err
}

// FindByMint finds a pool by mint.
func (r *PoolRepository) FindByMint(mint solana.PublicKey) (*domain.Pool, error) {
	pool := &domain.Pool{}
	query := `SELECT * FROM pools WHERE mint = $1`
	err := r.db.db.Get(pool, query, mint.String())
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return pool, err
}

// GetActivePools gets all active pools.
func (r *PoolRepository) GetActivePools() ([]*domain.Pool, error) {
	var pools []*domain.Pool
	query := `SELECT * FROM pools WHERE is_active = TRUE`
	err := r.db.db.Select(&pools, query)
	return pools, err
}

// GetAll gets all pools.
func (r *PoolRepository) GetAll() ([]*domain.Pool, error) {
	var pools []*domain.Pool
	query := `SELECT * FROM pools ORDER BY created_at DESC`
	err := r.db.db.Select(&pools, query)
	return pools, err
}

// RelayerRepository implements domain.RelayerRepository.
type RelayerRepository struct {
	db *PostgresDB
}

// NewRelayerRepository creates a new relayer repository.
func NewRelayerRepository(db *PostgresDB) *RelayerRepository {
	return &RelayerRepository{db: db}
}

// Create creates a new relayer.
func (r *RelayerRepository) Create(relayer *domain.Relayer) error {
	query := `
		INSERT INTO relayers (id, public_key, fee_account, is_active, total_relayed, total_fees_earned, total_errors, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.db.Exec(query,
		relayer.ID, relayer.PublicKey.String(), relayer.FeeAccount.String(),
		relayer.IsActive, relayer.TotalRelayed, relayer.TotalFeesEarned,
		relayer.TotalErrors, relayer.CreatedAt, relayer.UpdatedAt,
	)
	return err
}

// Update updates a relayer.
func (r *RelayerRepository) Update(relayer *domain.Relayer) error {
	query := `
		UPDATE relayers 
		SET total_relayed = $2, total_fees_earned = $3, total_errors = $4, is_active = $5, updated_at = $6
		WHERE public_key = $1
	`
	_, err := r.db.db.Exec(query,
		relayer.PublicKey.String(), relayer.TotalRelayed,
		relayer.TotalFeesEarned, relayer.TotalErrors,
		relayer.IsActive, relayer.UpdatedAt,
	)
	return err
}

// FindByPublicKey finds a relayer by public key.
func (r *RelayerRepository) FindByPublicKey(publicKey solana.PublicKey) (*domain.Relayer, error) {
	relayer := &domain.Relayer{}
	query := `SELECT * FROM relayers WHERE public_key = $1`
	err := r.db.db.Get(relayer, query, publicKey.String())
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return relayer, err
}

// GetActive gets the active relayer.
func (r *RelayerRepository) GetActive() (*domain.Relayer, error) {
	relayer := &domain.Relayer{}
	query := `SELECT * FROM relayers WHERE is_active = TRUE LIMIT 1`
	err := r.db.db.Get(relayer, query)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return relayer, err
}
