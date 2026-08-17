package pattern

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

type TokenManager struct {
	secret []byte
}

func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{secret: []byte(secret)}
}

func (tm *TokenManager) Generate(userID string, ttl time.Duration) (string, error) {
	payload := fmt.Sprintf("%s:%d", userID, time.Now().Add(ttl).Unix())
	sig := tm.sign(payload)
	return hex.EncodeToString([]byte(payload)) + "." + sig, nil
}

func (tm *TokenManager) Validate(token string) (string, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid token format")
	}
	payload, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid token encoding")
	}
	expectedSig := tm.sign(string(payload))
	if parts[1] != expectedSig {
		return "", fmt.Errorf("invalid token signature")
	}
	parts2 := strings.SplitN(string(payload), ":", 2)
	if len(parts2) != 2 {
		return "", fmt.Errorf("invalid token payload")
	}
	return parts2[0], nil
}

func (tm *TokenManager) sign(payload string) string {
	h := hmac.New(sha256.New, tm.secret)
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

type RetryableOperation func() error

type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	OnRetry      func(attempt int, err error)
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	}
}

func (rp RetryPolicy) Execute(op RetryableOperation) error {
	var lastErr error
	delay := rp.InitialDelay
	for i := 0; i < rp.MaxAttempts; i++ {
		if i > 0 {
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * rp.Multiplier)
			if delay > rp.MaxDelay {
				delay = rp.MaxDelay
			}
		}
		lastErr = op()
		if lastErr == nil {
			return nil
		}
		if rp.OnRetry != nil {
			rp.OnRetry(i+1, lastErr)
		}
	}
	return lastErr
}

type CircuitState struct {
	Name            string
	TotalRequests   int64
	FailureCount    int64
	SuccessCount    int64
	LastFailureTime time.Time
	LastStateChange time.Time
}

type ServiceHealth struct {
	Services    map[string]*ServiceHealthEntry
	mu          sync.RWMutex
}

type ServiceHealthEntry struct {
	Name        string
	Status      string
	Latency     time.Duration
	Error       string
	LastChecked time.Time
	ConsecutiveFails int
}

func NewServiceHealth() *ServiceHealth {
	return &ServiceHealth{Services: make(map[string]*ServiceHealthEntry)}
}

func (sh *ServiceHealth) Record(name string, latency time.Duration, err error) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	entry, ok := sh.Services[name]
	if !ok {
		entry = &ServiceHealthEntry{Name: name}
		sh.Services[name] = entry
	}
	entry.Latency = latency
	entry.LastChecked = time.Now()
	if err != nil {
		entry.Error = err.Error()
		entry.Status = "unhealthy"
		entry.ConsecutiveFails++
	} else {
		entry.Error = ""
		entry.Status = "healthy"
		entry.ConsecutiveFails = 0
	}
}

func (sh *ServiceHealth) IsHealthy(name string) bool {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	entry, ok := sh.Services[name]
	if !ok {
		return false
	}
	return entry.Status == "healthy"
}

func (sh *ServiceHealth) Snapshot() map[string]ServiceHealthEntry {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	result := make(map[string]ServiceHealthEntry)
	for k, v := range sh.Services {
		result[k] = *v
	}
	return result
}

type IDGenerator struct {
	prefix string
	seq    int64
	mu     sync.Mutex
}

func NewIDGenerator(prefix string) *IDGenerator {
	return &IDGenerator{prefix: prefix}
}

func (g *IDGenerator) Next() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	return fmt.Sprintf("%s_%d_%d", g.prefix, time.Now().UnixNano(), g.seq)
}
