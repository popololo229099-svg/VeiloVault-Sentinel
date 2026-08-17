package lifecycle

import (
	"fmt"
	"sync"
	"time"
)

type Plugin interface {
	Name() string
	Version() string
	Init(config map[string]interface{}) error
	Start() error
	Stop() error
	Health() error
}

type PluginManager struct {
	plugins map[string]Plugin
	mu      sync.RWMutex
}

func NewPluginManager() *PluginManager {
	return &PluginManager{plugins: make(map[string]Plugin)}
}

func (pm *PluginManager) Register(plugin Plugin) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if _, exists := pm.plugins[plugin.Name()]; exists {
		return fmt.Errorf("plugin %s already registered", plugin.Name())
	}
	pm.plugins[plugin.Name()] = plugin
	return nil
}

func (pm *PluginManager) Init(name string, config map[string]interface{}) error {
	pm.mu.RLock()
	plugin, ok := pm.plugins[name]
	pm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	return plugin.Init(config)
}

func (pm *PluginManager) Start(name string) error {
	pm.mu.RLock()
	plugin, ok := pm.plugins[name]
	pm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	return plugin.Start()
}

func (pm *PluginManager) Stop(name string) error {
	pm.mu.RLock()
	plugin, ok := pm.plugins[name]
	pm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	return plugin.Stop()
}

func (pm *PluginManager) StartAll() error {
	pm.mu.RLock()
	names := make([]string, 0, len(pm.plugins))
	for name := range pm.plugins {
		names = append(names, name)
	}
	pm.mu.RUnlock()

	for _, name := range names {
		if err := pm.Start(name); err != nil {
			return fmt.Errorf("start plugin %s: %w", name, err)
		}
	}
	return nil
}

func (pm *PluginManager) StopAll() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	for _, plugin := range pm.plugins {
		_ = plugin.Stop()
	}
}

func (pm *PluginManager) HealthAll() map[string]error {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	results := make(map[string]error)
	for name, plugin := range pm.plugins {
		results[name] = plugin.Health()
	}
	return results
}

type BasePlugin struct {
	name    string
	version string
	running bool
}

func (p *BasePlugin) Name() string    { return p.name }
func (p *BasePlugin) Version() string { return p.version }
func (p *BasePlugin) Health() error   { return nil }
func (p *BasePlugin) Stop() error     { p.running = false; return nil }

type RelayerPlugin struct {
	BasePlugin
	config map[string]interface{}
}

func NewRelayerPlugin() *RelayerPlugin {
	return &RelayerPlugin{
		BasePlugin: BasePlugin{name: "relayer", version: "1.0.0"},
	}
}

func (p *RelayerPlugin) Init(config map[string]interface{}) error {
	p.config = config
	return nil
}

func (p *RelayerPlugin) Start() error {
	p.running = true
	return nil
}

type MetricsPlugin struct {
	BasePlugin
	interval time.Duration
	stopCh   chan struct{}
}

func NewMetricsPlugin() *MetricsPlugin {
	return &MetricsPlugin{
		BasePlugin: BasePlugin{name: "metrics", version: "1.0.0"},
		interval:   10 * time.Second,
		stopCh:     make(chan struct{}),
	}
}

func (p *MetricsPlugin) Init(config map[string]interface{}) error {
	return nil
}

func (p *MetricsPlugin) Start() error {
	p.running = true
	go p.collect()
	return nil
}

func (p *MetricsPlugin) collect() {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
		}
	}
}

func (p *MetricsPlugin) Stop() error {
	p.running = false
	close(p.stopCh)
	return nil
}
