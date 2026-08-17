package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config interface {
	Get(key string) interface{}
	GetString(key string) string
	GetInt(key string) int
	GetInt64(key string) int64
	GetFloat64(key string) float64
	GetBool(key string) bool
	GetDuration(key string) time.Duration
	GetStringSlice(key string) []string
	Set(key string, value interface{})
	Has(key string) bool
	AllKeys() []string
}

type MapConfig struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

func NewMapConfig(data map[string]interface{}) *MapConfig {
	if data == nil {
		data = make(map[string]interface{})
	}
	return &MapConfig{data: data}
}

func (mc *MapConfig) Get(key string) interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.getNested(key)
}

func (mc *MapConfig) GetString(key string) string {
	val := mc.Get(key)
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%v", val)
}

func (mc *MapConfig) GetInt(key string) int {
	val := mc.Get(key)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func (mc *MapConfig) GetInt64(key string) int64 {
	val := mc.Get(key)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}

func (mc *MapConfig) GetFloat64(key string) float64 {
	val := mc.Get(key)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		n, _ := strconv.ParseFloat(v, 64)
		return n
	}
	return 0
}

func (mc *MapConfig) GetBool(key string) bool {
	val := mc.Get(key)
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	case int:
		return v != 0
	}
	return false
}

func (mc *MapConfig) GetDuration(key string) time.Duration {
	val := mc.Get(key)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case time.Duration:
		return v
	case string:
		d, _ := time.ParseDuration(v)
		return d
	case int:
		return time.Duration(v) * time.Second
	case int64:
		return time.Duration(v) * time.Second
	case float64:
		return time.Duration(v) * time.Second
	}
	return 0
}

func (mc *MapConfig) GetStringSlice(key string) []string {
	val := mc.Get(key)
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []string:
		return v
	case []interface{}:
		result := make([]string, len(v))
		for i, item := range v {
			result[i] = fmt.Sprintf("%v", item)
		}
		return result
	case string:
		return strings.Split(v, ",")
	}
	return nil
}

func (mc *MapConfig) Set(key string, value interface{}) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.setNested(key, value)
}

func (mc *MapConfig) Has(key string) bool {
	return mc.Get(key) != nil
}

func (mc *MapConfig) AllKeys() []string {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.allKeys("", mc.data)
}

func (mc *MapConfig) allKeys(prefix string, data map[string]interface{}) []string {
	keys := make([]string, 0)
	for k, v := range data {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}
		keys = append(keys, fullKey)
		if subMap, ok := v.(map[string]interface{}); ok {
			keys = append(keys, mc.allKeys(fullKey, subMap)...)
		}
	}
	return keys
}

func (mc *MapConfig) getNested(key string) interface{} {
	parts := strings.Split(key, ".")
	current := mc.data

	for i, part := range parts {
		if i == len(parts)-1 {
			return current[part]
		}
		next, ok := current[part]
		if !ok {
			return nil
		}
		subMap, ok := next.(map[string]interface{})
		if !ok {
			return nil
		}
		current = subMap
	}
	return nil
}

func (mc *MapConfig) setNested(key string, value interface{}) {
	parts := strings.Split(key, ".")
	current := mc.data

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}
		next, ok := current[part]
		if !ok {
			subMap := make(map[string]interface{})
			current[part] = subMap
			current = subMap
			continue
		}
		subMap, ok := next.(map[string]interface{})
		if !ok {
			subMap = make(map[string]interface{})
			current[part] = subMap
		}
		current = subMap
	}
}

func (mc *MapConfig) Merge(other *MapConfig) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()
	mc.mergeMaps(mc.data, other.data)
}

func (mc *MapConfig) mergeMaps(dst, src map[string]interface{}) {
	for key, srcVal := range src {
		if dstVal, ok := dst[key]; ok {
			dstSubMap, dstIsMap := dstVal.(map[string]interface{})
			srcSubMap, srcIsMap := srcVal.(map[string]interface{})
			if dstIsMap && srcIsMap {
				mc.mergeMaps(dstSubMap, srcSubMap)
				continue
			}
		}
		dst[key] = srcVal
	}
}

func (mc *MapConfig) Clone() *MapConfig {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	newData := mc.cloneMap(mc.data)
	return &MapConfig{data: newData}
}

func (mc *MapConfig) cloneMap(data map[string]interface{}) map[string]interface{} {
	newMap := make(map[string]interface{})
	for k, v := range data {
		if subMap, ok := v.(map[string]interface{}); ok {
			newMap[k] = mc.cloneMap(subMap)
		} else {
			newMap[k] = v
		}
	}
	return newMap
}

func (mc *MapConfig) MarshalJSON() ([]byte, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return json.Marshal(mc.data)
}

func (mc *MapConfig) UnmarshalJSON(data []byte) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return json.Unmarshal(data, &mc.data)
}

type ConfigLoader struct {
	sources    []ConfigSource
	config     *MapConfig
	mu         sync.RWMutex
	watchers   []func(*MapConfig)
	autoReload bool
	interval   time.Duration
	stopCh     chan struct{}
}

type ConfigSource interface {
	Load() (map[string]interface{}, error)
	Name() string
}

type ConfigLoaderConfig struct {
	Sources    []ConfigSource
	AutoReload bool
	Interval   time.Duration
}

func NewConfigLoader(config ConfigLoaderConfig) *ConfigLoader {
	if config.Interval <= 0 {
		config.Interval = 30 * time.Second
	}
	return &ConfigLoader{
		sources:    config.Sources,
		config:     NewMapConfig(nil),
		autoReload: config.AutoReload,
		interval:   config.Interval,
		stopCh:     make(chan struct{}),
	}
}

func (cl *ConfigLoader) Load() error {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	for _, source := range cl.sources {
		data, err := source.Load()
		if err != nil {
			return fmt.Errorf("failed to load from %s: %w", source.Name(), err)
		}
		cl.config.Merge(NewMapConfig(data))
	}

	for _, watcher := range cl.watchers {
		watcher(cl.config)
	}

	return nil
}

func (cl *ConfigLoader) Config() *MapConfig {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	return cl.config
}

func (cl *ConfigLoader) OnChange(fn func(*MapConfig)) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.watchers = append(cl.watchers, fn)
}

func (cl *ConfigLoader) StartAutoReload() {
	if !cl.autoReload {
		return
	}
	go func() {
		ticker := time.NewTicker(cl.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cl.Load()
			case <-cl.stopCh:
				return
			}
		}
	}()
}

func (cl *ConfigLoader) Stop() {
	close(cl.stopCh)
}

type FileSource struct {
	path string
}

func NewFileSource(path string) *FileSource {
	return &FileSource{path: path}
}

func (fs *FileSource) Load() (map[string]interface{}, error) {
	data, err := os.ReadFile(fs.path)
	if err != nil {
		return nil, err
	}

	ext := filepath.Ext(fs.path)
	switch ext {
	case ".json":
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result, nil
	default:
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
}

func (fs *FileSource) Name() string {
	return "file:" + fs.path
}

type EnvSource struct {
	prefix string
	sep    string
}

func NewEnvSource(prefix string) *EnvSource {
	return &EnvSource{prefix: prefix, sep: "."}
}

func (es *EnvSource) Load() (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(parts[0])
		value := parts[1]
		if es.prefix != "" && !strings.HasPrefix(key, strings.ToLower(es.prefix)) {
			continue
		}
		if es.prefix != "" {
			key = strings.TrimPrefix(key, strings.ToLower(es.prefix))
			key = strings.TrimPrefix(key, es.sep)
		}
		result[key] = value
	}
	return result, nil
}

func (es *EnvSource) Name() string {
	return "env:" + es.prefix
}

type MapSource struct {
	data map[string]interface{}
}

func NewMapSource(data map[string]interface{}) *MapSource {
	return &MapSource{data: data}
}

func (ms *MapSource) Load() (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for k, v := range ms.data {
		result[k] = v
	}
	return result, nil
}

func (ms *MapSource) Name() string {
	return "map"
}

type ConfigValidator struct {
	rules []ValidationRule
	mu    sync.RWMutex
}

type ValidationRule struct {
	Key      string
	Required bool
	Type     string
	Min      *float64
	Max      *float64
	Pattern  string
	Custom   func(interface{}) error
}

func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		rules: make([]ValidationRule, 0),
	}
}

func (cv *ConfigValidator) AddRule(rule ValidationRule) {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	cv.rules = append(cv.rules, rule)
}

func (cv *ConfigValidator) Validate(config Config) error {
	cv.mu.RLock()
	defer cv.mu.RUnlock()

	for _, rule := range cv.rules {
		val := config.Get(rule.Key)

		if rule.Required && val == nil {
			return fmt.Errorf("required key missing: %s", rule.Key)
		}

		if val != nil && rule.Type != "" {
			if err := cv.checkType(rule.Key, val, rule.Type); err != nil {
				return err
			}
		}

		if val != nil && rule.Custom != nil {
			if err := rule.Custom(val); err != nil {
				return fmt.Errorf("validation failed for %s: %w", rule.Key, err)
			}
		}
	}
	return nil
}

func (cv *ConfigValidator) checkType(key string, val interface{}, expected string) error {
	actual := reflect.TypeOf(val).String()
	switch expected {
	case "string":
		if _, ok := val.(string); !ok {
			return fmt.Errorf("key %s: expected string, got %s", key, actual)
		}
	case "int":
		switch val.(type) {
		case int, int64, float64:
		default:
			return fmt.Errorf("key %s: expected int, got %s", key, actual)
		}
	case "bool":
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("key %s: expected bool, got %s", key, actual)
		}
	case "duration":
		switch v := val.(type) {
		case time.Duration:
			_ = v
		case string:
			if _, err := time.ParseDuration(v); err != nil {
				return fmt.Errorf("key %s: invalid duration: %w", key, err)
			}
		default:
			return fmt.Errorf("key %s: expected duration, got %s", key, actual)
		}
	}
	return nil
}

type ConfigDiff struct {
	Added   map[string]interface{}
	Removed []string
	Changed map[string]interface{}
}

func DiffConfigs(old, new Config) *ConfigDiff {
	diff := &ConfigDiff{
		Added:   make(map[string]interface{}),
		Removed: make([]string, 0),
		Changed: make(map[string]interface{}),
	}

	oldKeys := old.AllKeys()
	newKeys := new.AllKeys()

	oldSet := make(map[string]bool)
	for _, k := range oldKeys {
		oldSet[k] = true
	}

	newSet := make(map[string]bool)
	for _, k := range newKeys {
		newSet[k] = true
	}

	for _, k := range newKeys {
		if !oldSet[k] {
			diff.Added[k] = new.Get(k)
		} else if fmt.Sprintf("%v", old.Get(k)) != fmt.Sprintf("%v", new.Get(k)) {
			diff.Changed[k] = new.Get(k)
		}
	}

	for _, k := range oldKeys {
		if !newSet[k] {
			diff.Removed = append(diff.Removed, k)
		}
	}

	return diff
}

func (cd *ConfigDiff) HasChanges() bool {
	return len(cd.Added) > 0 || len(cd.Removed) > 0 || len(cd.Changed) > 0
}

type ConfigEncryption struct {
	key      []byte
	algorithm string
	mu       sync.RWMutex
}

func NewConfigEncryption(key []byte, algorithm string) *ConfigEncryption {
	return &ConfigEncryption{
		key:       key,
		algorithm: algorithm,
	}
}

func (ce *ConfigEncryption) Encrypt(data []byte) ([]byte, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = b ^ ce.key[i%len(ce.key)]
	}
	return result, nil
}

func (ce *ConfigEncryption) Decrypt(data []byte) ([]byte, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = b ^ ce.key[i%len(ce.key)]
	}
	return result, nil
}

type HierarchicalConfig struct {
	layers   []*MapConfig
	overrides *MapConfig
	mu       sync.RWMutex
}

func NewHierarchicalConfig(layers ...*MapConfig) *HierarchicalConfig {
	return &HierarchicalConfig{
		layers:    layers,
		overrides: NewMapConfig(nil),
	}
}

func (hc *HierarchicalConfig) Get(key string) interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if val := hc.overrides.Get(key); val != nil {
		return val
	}

	for i := len(hc.layers) - 1; i >= 0; i-- {
		if val := hc.layers[i].Get(key); val != nil {
			return val
		}
	}
	return nil
}

func (hc *HierarchicalConfig) GetString(key string) string {
	val := hc.Get(key)
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%v", val)
}

func (hc *HierarchicalConfig) GetInt(key string) int {
	val := hc.Get(key)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}

func (hc *HierarchicalConfig) GetInt64(key string) int64 {
	return int64(hc.GetInt(key))
}

func (hc *HierarchicalConfig) GetFloat64(key string) float64 {
	val := hc.Get(key)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0
}

func (hc *HierarchicalConfig) GetBool(key string) bool {
	val := hc.Get(key)
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	}
	return false
}

func (hc *HierarchicalConfig) GetDuration(key string) time.Duration {
	val := hc.Get(key)
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case string:
		d, _ := time.ParseDuration(v)
		return d
	case int:
		return time.Duration(v) * time.Second
	}
	return 0
}

func (hc *HierarchicalConfig) GetStringSlice(key string) []string {
	val := hc.Get(key)
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []string:
		return v
	case string:
		return strings.Split(v, ",")
	}
	return nil
}

func (hc *HierarchicalConfig) Set(key string, value interface{}) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.overrides.Set(key, value)
}

func (hc *HierarchicalConfig) Has(key string) bool {
	return hc.Get(key) != nil
}

func (hc *HierarchicalConfig) AllKeys() []string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	keySet := make(map[string]bool)
	for i := len(hc.layers) - 1; i >= 0; i-- {
		for _, key := range hc.layers[i].AllKeys() {
			keySet[key] = true
		}
	}
	for _, key := range hc.overrides.AllKeys() {
		keySet[key] = true
	}

	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	return keys
}

func (hc *HierarchicalConfig) AddLayer(config *MapConfig) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.layers = append(hc.layers, config)
}
