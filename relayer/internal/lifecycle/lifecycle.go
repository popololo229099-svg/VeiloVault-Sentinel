package lifecycle

import (
	"context"
	"sync"
	"time"
)

type Hook func(ctx context.Context) error

type ShutdownHook func(ctx context.Context)

type GracefulManager struct {
	beforeStart  []Hook
	afterStart   []Hook
	beforeStop   []Hook
	afterStop    []ShutdownHook
	mu           sync.Mutex
	started      bool
	stopTimeout  time.Duration
}

func NewGracefulManager(stopTimeout time.Duration) *GracefulManager {
	return &GracefulManager{stopTimeout: stopTimeout}
}

func (gm *GracefulManager) BeforeStart(h Hook) { gm.mu.Lock(); gm.beforeStart = append(gm.beforeStart, h); gm.mu.Unlock() }
func (gm *GracefulManager) AfterStart(h Hook)  { gm.mu.Lock(); gm.afterStart = append(gm.afterStart, h); gm.mu.Unlock() }
func (gm *GracefulManager) BeforeStop(h Hook)  { gm.mu.Lock(); gm.beforeStop = append(gm.beforeStop, h); gm.mu.Unlock() }
func (gm *GracefulManager) AfterStop(h ShutdownHook) { gm.mu.Lock(); gm.afterStop = append(gm.afterStop, h); gm.mu.Unlock() }

func (gm *GracefulManager) Start(ctx context.Context) error {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	for _, h := range gm.beforeStart {
		if err := h(ctx); err != nil {
			return err
		}
	}
	gm.started = true
	for _, h := range gm.afterStart {
		_ = h(ctx)
	}
	return nil
}

func (gm *GracefulManager) Stop(ctx context.Context) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	for _, h := range gm.beforeStop {
		_ = h(ctx)
	}
	ctx, cancel := context.WithTimeout(ctx, gm.stopTimeout)
	defer cancel()
	_ = ctx
	for _, h := range gm.afterStop {
		h(ctx)
	}
	gm.started = false
}

func (gm *GracefulManager) IsStarted() bool {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	return gm.started
}

type FeatureFlags struct {
	flags map[string]*FeatureFlag
	mu    sync.RWMutex
}

type FeatureFlag struct {
	Name        string
	Enabled     bool
	Description string
	RolloutPct  float64
	UpdatedAt   time.Time
}

func NewFeatureFlags() *FeatureFlags {
	return &FeatureFlags{flags: make(map[string]*FeatureFlag)}
}

func (ff *FeatureFlags) IsEnabled(name string) bool {
	ff.mu.RLock()
	defer ff.mu.RUnlock()
	flag, ok := ff.flags[name]
	if !ok {
		return false
	}
	return flag.Enabled
}

func (ff *FeatureFlags) Set(name string, enabled bool, description string) {
	ff.mu.Lock()
	defer ff.mu.Unlock()
	ff.flags[name] = &FeatureFlag{
		Name:        name,
		Enabled:     enabled,
		Description: description,
		UpdatedAt:   time.Now(),
	}
}

func (ff *FeatureFlags) Get(name string) *FeatureFlag {
	ff.mu.RLock()
	defer ff.mu.RUnlock()
	return ff.flags[name]
}

func (ff *FeatureFlags) GetAll() map[string]*FeatureFlag {
	ff.mu.RLock()
	defer ff.mu.RUnlock()
	result := make(map[string]*FeatureFlag)
	for k, v := range ff.flags {
		result[k] = v
	}
	return result
}

type SecretManager struct {
	secrets map[string]string
	mu      sync.RWMutex
}

func NewSecretManager() *SecretManager {
	return &SecretManager{secrets: make(map[string]string)}
}

func (sm *SecretManager) Set(key, value string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.secrets[key] = value
}

func (sm *SecretManager) Get(key string) (string, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	val, ok := sm.secrets[key]
	return val, ok
}

func (sm *SecretManager) MustGet(key string) string {
	val, ok := sm.Get(key)
	if !ok {
		return ""
	}
	return val
}

func (sm *SecretManager) Delete(key string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.secrets, key)
}

func (sm *SecretManager) Keys() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	keys := make([]string, 0, len(sm.secrets))
	for k := range sm.secrets {
		keys = append(keys, k)
	}
	return keys
}

type ConfigWatcher struct {
	watchers map[string]func(string)
	configs  map[string]string
	mu       sync.RWMutex
}

func NewConfigWatcher() *ConfigWatcher {
	return &ConfigWatcher{
		watchers: make(map[string]func(string)),
		configs:  make(map[string]string),
	}
}

func (cw *ConfigWatcher) Watch(key string, handler func(string)) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.watchers[key] = handler
}

func (cw *ConfigWatcher) Set(key, value string) {
	cw.mu.Lock()
	cw.configs[key] = value
	handler, ok := cw.watchers[key]
	cw.mu.Unlock()
	if ok {
		handler(value)
	}
}

func (cw *ConfigWatcher) Get(key string) string {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.configs[key]
}
