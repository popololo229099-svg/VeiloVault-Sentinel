package data

import (
	"time"
)

type AuditEntry struct {
	ID        string
	Action    string
	Resource  string
	UserID    string
	Details   map[string]interface{}
	Timestamp time.Time
	IPAddress string
	UserAgent string
}

type AuditLog struct {
	entries []AuditEntry
	maxSize int
}

func NewAuditLog(maxSize int) *AuditLog {
	return &AuditLog{entries: make([]AuditEntry, 0), maxSize: maxSize}
}

func (al *AuditLog) Record(entry AuditEntry) {
	entry.Timestamp = time.Now()
	if len(al.entries) >= al.maxSize {
		al.entries = al.entries[1:]
	}
	al.entries = append(al.entries, entry)
}

func (al *AuditLog) Recent(n int) []AuditEntry {
	if n > len(al.entries) {
		n = len(al.entries)
	}
	result := make([]AuditEntry, n)
	copy(result, al.entries[len(al.entries)-n:])
	return result
}

func (al *AuditLog) ByUser(userID string) []AuditEntry {
	var result []AuditEntry
	for _, e := range al.entries {
		if e.UserID == userID {
			result = append(result, e)
		}
	}
	return result
}

func (al *AuditLog) ByAction(action string) []AuditEntry {
	var result []AuditEntry
	for _, e := range al.entries {
		if e.Action == action {
			result = append(result, e)
		}
	}
	return result
}

func (al *AuditLog) Count() int { return len(al.entries) }

type CircuitInfo struct {
	ID           string
	Name         string
	MintAddress  string
	VaultAddress string
	IsActive     bool
	CreatedAt    time.Time
}

type TransactionRecord struct {
	ID          string
	Type        string
	Status      string
	Signature   string
	PoolAddress string
	Amount      uint64
	Fee         uint64
	From        string
	To          string
	Slot        uint64
	Error       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type RateLimitRecord struct {
	Key       string
	Count     int
	Window    time.Time
	Limit     int
	Remaining int
}

type HealthCheckRecord struct {
	Service    string
	Status     string
	Latency    time.Duration
	Error      string
	CheckedAt  time.Time
}

type ErrorRecord struct {
	ID        string
	Error     string
	Level     string
	Source    string
	Count     int
	FirstSeen time.Time
	LastSeen  time.Time
	Context   map[string]interface{}
}
