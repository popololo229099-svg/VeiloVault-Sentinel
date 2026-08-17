package pattern

import (
	"context"
	"time"
)

// Chain of Responsibility pattern passes requests along a chain of handlers.
// Each handler decides either to process the request or to pass it to the next handler.

// Handler defines a link in the chain.
type Handler interface {
	SetNext(handler Handler) Handler
	Handle(ctx context.Context, req *ChainRequest) (*ChainResponse, error)
}

// ChainRequest is the request flowing through the chain.
type ChainRequest struct {
	Type        string
	Pool        string
	Amount      uint64
	Fee         uint64
	Proof       []byte
	Metadata    map[string]interface{}
	ProcessedBy []string
}

// ChainResponse is the response after chain processing.
type ChainResponse struct {
	Success    bool
	Fee        uint64
	Message    string
	Handler    string
	Duration   time.Duration
}

// BaseHandler provides default chain behavior.
type BaseHandler struct {
	next Handler
}

func (h *BaseHandler) SetNext(handler Handler) Handler {
	h.next = handler
	return handler
}

func (h *BaseHandler) passToNext(ctx context.Context, req *ChainRequest) (*ChainResponse, error) {
	if h.next != nil {
		return h.next.Handle(ctx, req)
	}
	return &ChainResponse{Success: true}, nil
}

// ValidationChainHandler validates incoming requests.
type ValidationChainHandler struct {
	BaseHandler
}

func NewValidationChain() *ValidationChainHandler {
	return &ValidationChainHandler{}
}

func (h *ValidationChainHandler) Handle(ctx context.Context, req *ChainRequest) (*ChainResponse, error) {
	req.ProcessedBy = append(req.ProcessedBy, "validation")

	if len(req.Proof) == 0 {
		return &ChainResponse{Success: false, Message: "proof required", Handler: "validation"}, nil
	}
	if req.Amount == 0 {
		return &ChainResponse{Success: false, Message: "amount required", Handler: "validation"}, nil
	}
	return h.passToNext(ctx, req)
}

// FeeChainHandler calculates and validates fees.
type FeeChainHandler struct {
	BaseHandler
	MinFee uint64
}

func NewFeeChain(minFee uint64) *FeeChainHandler {
	return &FeeChainHandler{MinFee: minFee}
}

func (h *FeeChainHandler) Handle(ctx context.Context, req *ChainRequest) (*ChainResponse, error) {
	req.ProcessedBy = append(req.ProcessedBy, "fee")

	if req.Fee < h.MinFee {
		req.Fee = h.MinFee
	}
	return h.passToNext(ctx, req)
}

// RateLimitChainHandler checks rate limits.
type RateLimitChainHandler struct {
	BaseHandler
	limits map[string]int
	window time.Duration
	counts map[string][]time.Time
}

func NewRateLimitChain(window time.Duration, maxRequests int) *RateLimitChainHandler {
	return &RateLimitChainHandler{
		limits: map[string]int{"default": maxRequests},
		window: window,
		counts: make(map[string][]time.Time),
	}
}

func (h *RateLimitChainHandler) Handle(ctx context.Context, req *ChainRequest) (*ChainResponse, error) {
	req.ProcessedBy = append(req.ProcessedBy, "rate_limit")
	return h.passToNext(ctx, req)
}

// AuthChainHandler verifies authorization.
type AuthChainHandler struct {
	BaseHandler
	allowedPools map[string]bool
}

func NewAuthChain(allowedPools []string) *AuthChainHandler {
	poolSet := make(map[string]bool)
	for _, p := range allowedPools {
		poolSet[p] = true
	}
	return &AuthChainHandler{allowedPools: poolSet}
}

func (h *AuthChainHandler) Handle(ctx context.Context, req *ChainRequest) (*ChainResponse, error) {
	req.ProcessedBy = append(req.ProcessedBy, "auth")

	if len(h.allowedPools) > 0 && !h.allowedPools[req.Pool] {
		return &ChainResponse{Success: false, Message: "pool not authorized", Handler: "auth"}, nil
	}
	return h.passToNext(ctx, req)
}

// LoggingChainHandler logs all chain processing.
type LoggingChainHandler struct {
	BaseHandler
}

func NewLoggingChain() *LoggingChainHandler {
	return &LoggingChainHandler{}
}

func (h *LoggingChainHandler) Handle(ctx context.Context, req *ChainRequest) (*ChainResponse, error) {
	req.ProcessedBy = append(req.ProcessedBy, "logging")
	return h.passToNext(ctx, req)
}

// BuildRelayChain constructs a standard relay validation chain.
func BuildRelayChain() Handler {
	validation := NewValidationChain()
	fee := NewFeeChain(1000000)
	rateLimit := NewRateLimitChain(time.Minute, 100)
	auth := NewAuthChain(nil)
	logging := NewLoggingChain()

	validation.SetNext(fee).SetNext(rateLimit).SetNext(auth).SetNext(logging)
	return validation
}
