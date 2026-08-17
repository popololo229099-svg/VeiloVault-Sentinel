// Package pattern implements GoF design patterns for the relayer.
// Strategy: Defines a family of fee calculation algorithms, encapsulates each one,
// and makes them interchangeable. The FeeCalculator varies independently from
// the relay use case that uses it.
//
// Ref: https://refactoring.guru/design-patterns/strategy
package pattern

import (
	"math"

	"github.com/gagliardetto/solana-go"
)

// FeeStrategy defines the interface for fee calculation algorithms.
type FeeStrategy interface {
	Calculate(params FeeParams) uint64
	Name() string
}

// FeeParams holds inputs for fee calculation.
type FeeParams struct {
	Amount       uint64
	Pool         solana.PublicKey
	IsNativeSOL  bool
	Slot         uint64
	BaseFee      uint64
	FeeBPS       uint16
	MinFee       uint64
	MaxFee       uint64
}

// FlatFeeStrategy charges a fixed fee regardless of amount.
type FlatFeeStrategy struct {
	FlatAmount uint64
}

func NewFlatFeeStrategy(flatAmount uint64) *FlatFeeStrategy {
	return &FlatFeeStrategy{FlatAmount: flatAmount}
}

func (s *FlatFeeStrategy) Calculate(params FeeParams) uint64 {
	fee := s.FlatAmount
	if fee < params.MinFee {
		fee = params.MinFee
	}
	if fee > params.MaxFee && params.MaxFee > 0 {
		fee = params.MaxFee
	}
	return fee
}

func (s *FlatFeeStrategy) Name() string { return "flat" }

// PercentageFeeStrategy charges a percentage of the transaction amount.
type PercentageFeeStrategy struct {
	BasisPoints uint16
}

func NewPercentageFeeStrategy(bps uint16) *PercentageFeeStrategy {
	return &PercentageFeeStrategy{BasisPoints: bps}
}

func (s *PercentageFeeStrategy) Calculate(params FeeParams) uint64 {
	fee := uint64(float64(params.Amount) * float64(s.BasisPoints) / 10000.0)
	if fee < params.MinFee {
		fee = params.MinFee
	}
	if params.MaxFee > 0 && fee > params.MaxFee {
		fee = params.MaxFee
	}
	return fee
}

func (s *PercentageFeeStrategy) Name() string { return "percentage" }

// TieredFeeStrategy charges different rates based on amount tiers.
type TieredFeeStrategy struct {
	Tiers []FeeTier
}

type FeeTier struct {
	MinAmount uint64
	MaxAmount uint64
	BPS       uint16
}

func NewTieredFeeStrategy(tiers []FeeTier) *TieredFeeStrategy {
	return &TieredFeeStrategy{Tiers: tiers}
}

func (s *TieredFeeStrategy) Calculate(params FeeParams) uint64 {
	for _, tier := range s.Tiers {
		if params.Amount >= tier.MinAmount && (tier.MaxAmount == 0 || params.Amount <= tier.MaxAmount) {
			fee := uint64(float64(params.Amount) * float64(tier.BPS) / 10000.0)
			if fee < params.MinFee {
				fee = params.MinFee
			}
			return fee
		}
	}
	return params.MinFee
}

func (s *TieredFeeStrategy) Name() string { return "tiered" }

// DynamicFeeStrategy adjusts fees based on network congestion (slot delta).
type DynamicFeeStrategy struct {
	BaseBPS     uint16
	MinBPS      uint16
	MaxBPS      uint16
	CongestionThreshold uint64
}

func NewDynamicFeeStrategy(baseBPS uint16) *DynamicFeeStrategy {
	return &DynamicFeeStrategy{
		BaseBPS:             baseBPS,
		MinBPS:              baseBPS / 2,
		MaxBPS:              baseBPS * 3,
		CongestionThreshold: 100,
	}
}

func (s *DynamicFeeStrategy) Calculate(params FeeParams) uint64 {
	bps := s.BaseBPS
	if params.Slot > s.CongestionThreshold {
		ratio := float64(params.Slot-s.CongestionThreshold) / float64(s.CongestionThreshold)
		bps = s.MinBPS + uint16(float64(s.MaxBPS-s.MinBPS)*math.Min(ratio, 1.0))
	}

	fee := uint64(float64(params.Amount) * float64(bps) / 10000.0)
	if fee < params.MinFee {
		fee = params.MinFee
	}
	if params.MaxFee > 0 && fee > params.MaxFee {
		fee = params.MaxFee
	}
	return fee
}

func (s *DynamicFeeStrategy) Name() string { return "dynamic" }

// CompositeFeeStrategy combines multiple strategies with AND/OR logic.
type CompositeFeeStrategy struct {
	Strategies []FeeStrategy
	Mode       CompositeMode
}

type CompositeMode int

const (
	CompositeMin CompositeMode = iota
	CompositeMax
	CompositeAverage
)

func NewCompositeFeeStrategy(mode CompositeMode, strategies ...FeeStrategy) *CompositeFeeStrategy {
	return &CompositeFeeStrategy{Strategies: strategies, Mode: mode}
}

func (s *CompositeFeeStrategy) Calculate(params FeeParams) uint64 {
	if len(s.Strategies) == 0 {
		return params.MinFee
	}

	fees := make([]uint64, len(s.Strategies))
	for i, strategy := range s.Strategies {
		fees[i] = strategy.Calculate(params)
	}

	switch s.Mode {
	case CompositeMin:
		min := fees[0]
		for _, f := range fees[1:] {
			if f < min {
				min = f
			}
		}
		return min
	case CompositeMax:
		max := fees[0]
		for _, f := range fees[1:] {
			if f > max {
				max = f
			}
		}
		return max
	default:
		var sum uint64
		for _, f := range fees {
			sum += f
		}
		return sum / uint64(len(fees))
	}
}

func (s *CompositeFeeStrategy) Name() string { return "composite" }
