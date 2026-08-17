package pattern

import (
	"context"
	"sync"
	"time"
)

// State pattern allows an object to alter its behavior when its internal state changes.

// TransactionState defines the interface for transaction states.
type TransactionState interface {
	Enter(ctx context.Context, tx *StateTransaction)
	Submit(ctx context.Context, tx *StateTransaction) error
	Confirm(ctx context.Context, tx *StateTransaction) error
	Fail(ctx context.Context, tx *StateTransaction, err error) error
	Name() string
}

// StateTransaction is the context object that changes behavior based on state.
type StateTransaction struct {
	mu          sync.RWMutex
	state       TransactionState
	ID          string
	Signature   string
	Status      string
	Error       string
	SubmittedAt *time.Time
	ConfirmedAt *time.Time
	FailedAt    *time.Time
Transitions  []StateTransition
}

type StateTransition struct {
	From      string
	To        string
	Timestamp time.Time
}

func NewStateTransaction(id string) *StateTransaction {
	tx := &StateTransaction{ID: id, Status: "created"}
	tx.setState(&PendingState{})
	return tx
}

func (tx *StateTransaction) setState(state TransactionState) {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	old := tx.Status
	tx.state = state
	tx.Status = state.Name()
	tx.Transitions = append(tx.Transitions, StateTransition{
		From: old, To: tx.Status, Timestamp: time.Now(),
	})
}

func (tx *StateTransaction) Submit(ctx context.Context) error {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.state.Submit(ctx, tx)
}

func (tx *StateTransaction) Confirm(ctx context.Context) error {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.state.Confirm(ctx, tx)
}

func (tx *StateTransaction) Fail(ctx context.Context, err error) error {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.state.Fail(ctx, tx, err)
}

func (tx *StateTransaction) GetState() string {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	return tx.Status
}

func (tx *StateTransaction) GetTransitions() []StateTransition {
	tx.mu.RLock()
	defer tx.mu.RUnlock()
	result := make([]StateTransition, len(tx.Transitions))
	copy(result, tx.Transitions)
	return result
}

// --- Concrete States ---

type PendingState struct{}

func (s *PendingState) Enter(ctx context.Context, tx *StateTransaction) {}
func (s *PendingState) Name() string                                    { return "pending" }

func (s *PendingState) Submit(ctx context.Context, tx *StateTransaction) error {
	now := time.Now()
	tx.SubmittedAt = &now
	tx.setState(&SubmittedState{})
	return nil
}

func (s *PendingState) Confirm(ctx context.Context, tx *StateTransaction) error {
	return ErrInvalidStateTransition
}

func (s *PendingState) Fail(ctx context.Context, tx *StateTransaction, err error) error {
	tx.Error = err.Error()
	tx.setState(&FailedState{})
	return nil
}

type SubmittedState struct{}

func (s *SubmittedState) Enter(ctx context.Context, tx *StateTransaction) {}
func (s *SubmittedState) Name() string                                    { return "submitted" }

func (s *SubmittedState) Submit(ctx context.Context, tx *StateTransaction) error {
	return ErrAlreadySubmitted
}

func (s *SubmittedState) Confirm(ctx context.Context, tx *StateTransaction) error {
	now := time.Now()
	tx.ConfirmedAt = &now
	tx.setState(&ConfirmedState{})
	return nil
}

func (s *SubmittedState) Fail(ctx context.Context, tx *StateTransaction, err error) error {
	tx.Error = err.Error()
	tx.setState(&FailedState{})
	return nil
}

type ConfirmedState struct{}

func (s *ConfirmedState) Enter(ctx context.Context, tx *StateTransaction) {}
func (s *ConfirmedState) Name() string                                    { return "confirmed" }

func (s *ConfirmedState) Submit(ctx context.Context, tx *StateTransaction) error {
	return ErrAlreadyConfirmed
}

func (s *ConfirmedState) Confirm(ctx context.Context, tx *StateTransaction) error {
	return nil
}

func (s *ConfirmedState) Fail(ctx context.Context, tx *StateTransaction, err error) error {
	return ErrAlreadyConfirmed
}

type FailedState struct{}

func (s *FailedState) Enter(ctx context.Context, tx *StateTransaction) {}
func (s *FailedState) Name() string                                    { return "failed" }

func (s *FailedState) Submit(ctx context.Context, tx *StateTransaction) error {
	return ErrTransactionFailed
}

func (s *FailedState) Confirm(ctx context.Context, tx *StateTransaction) error {
	return ErrTransactionFailed
}

func (s *FailedState) Fail(ctx context.Context, tx *StateTransaction, err error) error {
	return nil
}
