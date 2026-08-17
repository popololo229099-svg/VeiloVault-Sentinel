package factory

import (
	"fmt"
	"sync"
	"time"
)

type StrategyType string

const (
	StrategyFlat       StrategyType = "flat"
	StrategyPercentage StrategyType = "percentage"
	StrategyTiered     StrategyType = "tiered"
	StrategyDynamic    StrategyType = "dynamic"
	StrategyComposite  StrategyType = "composite"
)

type FeeStrategy interface {
	Calculate(amount uint64) (uint64, error)
	Type() StrategyType
}

type FlatFee struct {
	fee uint64
}

func NewFlatFee(fee uint64) *FlatFee { return &FlatFee{fee: fee} }

func (f *FlatFee) Calculate(amount uint64) (uint64, error) { return f.fee, nil }
func (f *FlatFee) Type() StrategyType                      { return StrategyFlat }

type PercentageFee struct {
	bps uint16
}

func NewPercentageFee(bps uint16) *PercentageFee { return &PercentageFee{bps: bps} }

func (f *PercentageFee) Calculate(amount uint64) (uint64, error) {
	return amount * uint64(f.bps) / 10000, nil
}

func (f *PercentageFee) Type() StrategyType { return StrategyPercentage }

type TieredFee struct {
	tiers []FeeTier
}

type FeeTier struct {
	Min     uint64
	Max     uint64
	FeeBPS  uint16
}

func NewTieredFee(tiers []FeeTier) *TieredFee { return &TieredFee{tiers: tiers} }

func (f *TieredFee) Calculate(amount uint64) (uint64, error) {
	for _, tier := range f.tiers {
		if amount >= tier.Min && (tier.Max == 0 || amount < tier.Max) {
			return amount * uint64(tier.FeeBPS) / 10000, nil
		}
	}
	return 0, fmt.Errorf("no matching tier for amount %d", amount)
}

func (f *TieredFee) Type() StrategyType { return StrategyTiered }

type DynamicFee struct {
	baseBPS   uint16
	volatility float64
	mu         sync.RWMutex
}

func NewDynamicFee(baseBPS uint16) *DynamicFee {
	return &DynamicFee{baseBPS: baseBPS}
}

func (f *DynamicFee) UpdateVolatility(vol float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.volatility = vol
}

func (f *DynamicFee) Calculate(amount uint64) (uint64, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	multiplier := 1.0 + f.volatility
	effectiveBPS := float64(f.baseBPS) * multiplier
	return uint64(float64(amount) * effectiveBPS / 10000), nil
}

func (f *DynamicFee) Type() StrategyType { return StrategyDynamic }

type CompositeFee struct {
	strategies []FeeStrategy
	operation  string
}

func NewCompositeFee(operation string, strategies ...FeeStrategy) *CompositeFee {
	return &CompositeFee{strategies: strategies, operation: operation}
}

func (f *CompositeFee) Calculate(amount uint64) (uint64, error) {
	var total uint64
	for _, s := range f.strategies {
		fee, err := s.Calculate(amount)
		if err != nil {
			return 0, err
		}
		if f.operation == "sum" {
			total += fee
		} else if f.operation == "max" && fee > total {
			total = fee
		} else if f.operation == "min" && (total == 0 || fee < total) {
			total = fee
		}
	}
	return total, nil
}

func (f *CompositeFee) Type() StrategyType { return StrategyComposite }

type FeeStrategyFactory struct {
	strategies map[StrategyType]FeeStrategy
	mu         sync.RWMutex
}

func NewFeeStrategyFactory() *FeeStrategyFactory {
	f := &FeeStrategyFactory{strategies: make(map[StrategyType]FeeStrategy)}
	f.Register(StrategyFlat, NewFlatFee(0))
	f.Register(StrategyPercentage, NewPercentageFee(50))
	return f
}

func (f *FeeStrategyFactory) Register(t StrategyType, strategy FeeStrategy) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.strategies[t] = strategy
}

func (f *FeeStrategyFactory) Get(t StrategyType) (FeeStrategy, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if s, ok := f.strategies[t]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("unknown strategy: %s", t)
}

type FeeEstimate struct {
	Amount      uint64
	Fee         uint64
	Strategy    StrategyType
	EstimatedAt time.Time
}

type FeeCalculator struct {
	factory *FeeStrategyFactory
}

func NewFeeCalculator(factory *FeeStrategyFactory) *FeeCalculator {
	return &FeeCalculator{factory: factory}
}

func (fc *FeeCalculator) Estimate(amount uint64, strategyType StrategyType) (*FeeEstimate, error) {
	strategy, err := fc.factory.Get(strategyType)
	if err != nil {
		return nil, err
	}
	fee, err := strategy.Calculate(amount)
	if err != nil {
		return nil, err
	}
	return &FeeEstimate{
		Amount:      amount,
		Fee:         fee,
		Strategy:    strategyType,
		EstimatedAt: time.Now(),
	}, nil
}
