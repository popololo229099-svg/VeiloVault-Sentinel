package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

type Strategy interface {
	NextDelay(attempt int, lastDelay time.Duration) time.Duration
	Name() string
}

type ExponentialBackoff struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	Jitter       float64
	mu           sync.RWMutex
}

func NewExponentialBackoff(initial, max time.Duration, multiplier float64) *ExponentialBackoff {
	if multiplier <= 0 {
		multiplier = 2.0
	}
	return &ExponentialBackoff{
		InitialDelay: initial,
		MaxDelay:     max,
		Multiplier:   multiplier,
		Jitter:       0.1,
	}
}

func (eb *ExponentialBackoff) SetJitter(jitter float64) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.Jitter = jitter
}

func (eb *ExponentialBackoff) NextDelay(attempt int, _ time.Duration) time.Duration {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	delay := float64(eb.InitialDelay) * math.Pow(eb.Multiplier, float64(attempt))
	if delay > float64(eb.MaxDelay) {
		delay = float64(eb.MaxDelay)
	}

	if eb.Jitter > 0 {
		jitterRange := delay * eb.Jitter
		delay = delay - jitterRange + (rand.Float64() * 2 * jitterRange)
	}

	return time.Duration(delay)
}

func (eb *ExponentialBackoff) Name() string { return "exponential" }

type LinearBackoff struct {
	InitialDelay time.Duration
	Increment    time.Duration
	MaxDelay     time.Duration
	mu           sync.RWMutex
}

func NewLinearBackoff(initial, increment, max time.Duration) *LinearBackoff {
	return &LinearBackoff{
		InitialDelay: initial,
		Increment:    increment,
		MaxDelay:     max,
	}
}

func (lb *LinearBackoff) NextDelay(attempt int, _ time.Duration) time.Duration {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	delay := lb.InitialDelay + lb.Increment*time.Duration(attempt)
	if delay > lb.MaxDelay {
		return lb.MaxDelay
	}
	return delay
}

func (lb *LinearBackoff) Name() string { return "linear" }

type FixedDelay struct {
	Delay time.Duration
	mu    sync.RWMutex
}

func NewFixedDelay(delay time.Duration) *FixedDelay {
	return &FixedDelay{Delay: delay}
}

func (fd *FixedDelay) NextDelay(_ int, _ time.Duration) time.Duration {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	return fd.Delay
}

func (fd *FixedDelay) Name() string { return "fixed" }

type FibonacciBackoff struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	mu           sync.RWMutex
}

func NewFibonacciBackoff(initial, max time.Duration) *FibonacciBackoff {
	return &FibonacciBackoff{
		InitialDelay: initial,
		MaxDelay:     max,
	}
}

func (fb *FibonacciBackoff) NextDelay(attempt int, _ time.Duration) time.Duration {
	fb.mu.RLock()
	defer fb.mu.RUnlock()

	a, b := int64(1), int64(1)
	for i := 0; i < attempt+1; i++ {
		a, b = b, a+b
	}

	delay := time.Duration(int64(fb.InitialDelay) * a)
	if delay > fb.MaxDelay {
		return fb.MaxDelay
	}
	return delay
}

func (fb *FibonacciBackoff) Name() string { return "fibonacci" }

type Policy struct {
	MaxAttempts int
	Strategy    Strategy
	OnRetry     func(attempt int, err error, delay time.Duration)
	OnSuccess   func(attempt int)
	OnFailure   func(attempt int, err error)
	ShouldRetry func(err error) bool
	Timeout     time.Duration
	mu          sync.RWMutex
}

type PolicyConfig struct {
	MaxAttempts int
	Strategy    Strategy
	OnRetry     func(int, error, time.Duration)
	OnSuccess   func(int)
	OnFailure   func(int, error)
	ShouldRetry func(error) bool
	Timeout     time.Duration
}

func DefaultPolicy() *Policy {
	return &Policy{
		MaxAttempts: 3,
		Strategy:    NewExponentialBackoff(100*time.Millisecond, 10*time.Second, 2.0),
		Timeout:     30 * time.Second,
	}
}

func NewPolicy(config PolicyConfig) *Policy {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.Strategy == nil {
		config.Strategy = NewExponentialBackoff(100*time.Millisecond, 10*time.Second, 2.0)
	}
	return &Policy{
		MaxAttempts: config.MaxAttempts,
		Strategy:    config.Strategy,
		OnRetry:     config.OnRetry,
		OnSuccess:   config.OnSuccess,
		OnFailure:   config.OnFailure,
		ShouldRetry: config.ShouldRetry,
		Timeout:     config.Timeout,
	}
}

func (p *Policy) SetMaxAttempts(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.MaxAttempts = n
}

func (p *Policy) SetStrategy(s Strategy) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Strategy = s
}

func Execute(ctx context.Context, policy *Policy, fn func(ctx context.Context) error) error {
	policy.mu.RLock()
	maxAttempts := policy.MaxAttempts
	strategy := policy.Strategy
	timeout := policy.Timeout
	onRetry := policy.OnRetry
	onSuccess := policy.OnSuccess
	onFailure := policy.OnFailure
	shouldRetry := policy.ShouldRetry
	policy.mu.RUnlock()

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var lastDelay time.Duration
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled: %w", err)
		}

		err := fn(ctx)
		if err == nil {
			if onSuccess != nil {
				onSuccess(attempt)
			}
			return nil
		}

		lastErr = err

		if shouldRetry != nil && !shouldRetry(err) {
			break
		}

		if attempt < maxAttempts-1 {
			delay := strategy.NextDelay(attempt, lastDelay)
			lastDelay = delay

			if onRetry != nil {
				onRetry(attempt, err, delay)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	if onFailure != nil {
		onFailure(maxAttempts, lastErr)
	}

	return fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}

type RetryEngine struct {
	policies map[string]*Policy
	mu       sync.RWMutex
}

func NewRetryEngine() *RetryEngine {
	return &RetryEngine{
		policies: make(map[string]*Policy),
	}
}

func (re *RetryEngine) RegisterPolicy(name string, policy *Policy) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.policies[name] = policy
}

func (re *RetryEngine) GetPolicy(name string) *Policy {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.policies[name]
}

func (re *RetryEngine) Execute(ctx context.Context, policyName string, fn func(ctx context.Context) error) error {
	policy := re.GetPolicy(policyName)
	if policy == nil {
		return fmt.Errorf("policy not found: %s", policyName)
	}
	return Execute(ctx, policy, fn)
}

func (re *RetryEngine) RemovePolicy(name string) {
	re.mu.Lock()
	defer re.mu.Unlock()
	delete(re.policies, name)
}

func (re *RetryEngine) PolicyCount() int {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return len(re.policies)
}

func (re *RetryEngine) ListPolicies() []string {
	re.mu.RLock()
	defer re.mu.RUnlock()
	names := make([]string, 0, len(re.policies))
	for name := range re.policies {
		names = append(names, name)
	}
	return names
}

type RetryMetrics struct {
	TotalAttempts int64
	SuccessCount  int64
	FailureCount  int64
	RetryCount    int64
	TotalDelay    time.Duration
	mu            sync.RWMutex
}

func NewRetryMetrics() *RetryMetrics {
	return &RetryMetrics{}
}

func (rm *RetryMetrics) RecordAttempt() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.TotalAttempts++
}

func (rm *RetryMetrics) RecordSuccess() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.SuccessCount++
}

func (rm *RetryMetrics) RecordFailure() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.FailureCount++
}

func (rm *RetryMetrics) RecordRetry(delay time.Duration) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.RetryCount++
	rm.TotalDelay += delay
}

func (rm *RetryMetrics) Snapshot() RetryMetricsSnapshot {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return RetryMetricsSnapshot{
		TotalAttempts: rm.TotalAttempts,
		SuccessCount:  rm.SuccessCount,
		FailureCount:  rm.FailureCount,
		RetryCount:    rm.RetryCount,
		TotalDelay:    rm.TotalDelay,
	}
}

func (rm *RetryMetrics) Reset() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.TotalAttempts = 0
	rm.SuccessCount = 0
	rm.FailureCount = 0
	rm.RetryCount = 0
	rm.TotalDelay = 0
}

type RetryMetricsSnapshot struct {
	TotalAttempts int64
	SuccessCount  int64
	FailureCount  int64
	RetryCount    int64
	TotalDelay    time.Duration
}

type ConditionalRetry struct {
	condition func(error) bool
	policy    *Policy
	mu        sync.RWMutex
}

func NewConditionalRetry(condition func(error) bool, policy *Policy) *ConditionalRetry {
	return &ConditionalRetry{
		condition: condition,
		policy:    policy,
	}
}

func (cr *ConditionalRetry) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	cr.mu.RLock()
	policy := cr.policy
	condition := cr.condition
	cr.mu.RUnlock()

	originalShouldRetry := policy.ShouldRetry
	policy.mu.Lock()
	policy.ShouldRetry = func(err error) bool {
		if condition != nil && !condition(err) {
			return false
		}
		if originalShouldRetry != nil {
			return originalShouldRetry(err)
		}
		return true
	}
	policy.mu.Unlock()

	defer func() {
		policy.mu.Lock()
		policy.ShouldRetry = originalShouldRetry
		policy.mu.Unlock()
	}()

	return Execute(ctx, policy, fn)
}

type RetryLoop struct {
	policy   *Policy
	ctx      context.Context
	attempt  int
	lastErr  error
	finished bool
	mu       sync.Mutex
}

func NewRetryLoop(ctx context.Context, policy *Policy) *RetryLoop {
	return &RetryLoop{
		ctx:    ctx,
		policy: policy,
	}
}

func (rl *RetryLoop) Next() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.policy.mu.RLock()
	maxAttempts := rl.policy.MaxAttempts
	rl.policy.mu.RUnlock()

	if rl.finished || rl.attempt >= maxAttempts {
		return false
	}

	if rl.attempt > 0 {
		rl.policy.mu.RLock()
		strategy := rl.policy.Strategy
		rl.policy.mu.RUnlock()

		delay := strategy.NextDelay(rl.attempt-1, 0)
		select {
		case <-rl.ctx.Done():
			rl.finished = true
			return false
		case <-time.After(delay):
		}
	}

	rl.attempt++
	return true
}

func (rl *RetryLoop) RecordError(err error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.lastErr = err
}

func (rl *RetryLoop) Attempt() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.attempt
}

func (rl *RetryLoop) LastError() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.lastErr
}

type RetryMiddleware struct {
	policy *Policy
	mu     sync.RWMutex
}

func NewRetryMiddleware(policy *Policy) *RetryMiddleware {
	return &RetryMiddleware{policy: policy}
}

func (rm *RetryMiddleware) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	rm.mu.RLock()
	policy := rm.policy
	rm.mu.RUnlock()

	return Execute(ctx, policy, fn)
}

func (rm *RetryMiddleware) SetPolicy(policy *Policy) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.policy = policy
}

type BackoffCalculator struct {
	strategy Strategy
	counter  int64
	mu       sync.Mutex
}

func NewBackoffCalculator(strategy Strategy) *BackoffCalculator {
	return &BackoffCalculator{strategy: strategy}
}

func (bc *BackoffCalculator) Next() time.Duration {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	delay := bc.strategy.NextDelay(int(bc.counter), 0)
	bc.counter++
	return delay
}

func (bc *BackoffCalculator) Reset() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.counter = 0
}

func (bc *BackoffCalculator) Count() int64 {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.counter
}
