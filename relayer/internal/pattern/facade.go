package pattern

import (
	"context"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
)

// Facade pattern provides a simplified interface to a complex subsystem.

// FacadePool is a local pool type for the facade.
type FacadePool struct {
	Address  solana.PublicKey
	Mint     solana.PublicKey
	Vault    solana.PublicKey
	IsActive bool
	Balance  uint64
}

// SolanaSubsystem groups all Solana-related operations behind one interface.
type SolanaSubsystem struct {
	client   InternalRelayer
	metrics  *MetricsDecorator
	emitter  *EventEmitter
}

func NewSolanaSubsystem(client InternalRelayer) *SolanaSubsystem {
	return &SolanaSubsystem{
		client:  client,
		metrics: NewMetricsDecorator(&BaseProcessor{}),
		emitter: NewEventEmitter(),
	}
}

func (s *SolanaSubsystem) SubmitAndTrack(ctx context.Context, data []byte) (*FacadeResult, error) {
	start := time.Now()
	sig, err := s.client.SubmitTransaction(ctx, data)
	if err != nil {
		s.emitter.Emit(Event{Type: "submission.failed", Source: "facade"})
		return nil, err
	}
	s.emitter.Emit(Event{Type: "submission.success", Source: "facade", Payload: sig})
	return &FacadeResult{Signature: sig, Status: "submitted", Duration: time.Since(start)}, nil
}

type FacadeResult struct {
	Signature string
	Status    string
	Duration  time.Duration
}

type PoolFacade struct {
	pools map[string]*FacadePool
	mu    sync.RWMutex
}

func NewPoolFacade() *PoolFacade {
	return &PoolFacade{pools: make(map[string]*FacadePool)}
}

func (f *PoolFacade) RegisterPool(pool *FacadePool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pools[pool.Address.String()] = pool
}

func (f *PoolFacade) GetActivePools() []*FacadePool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var active []*FacadePool
	for _, p := range f.pools {
		if p.IsActive {
			active = append(active, p)
		}
	}
	return active
}

func (f *PoolFacade) PoolCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.pools)
}
