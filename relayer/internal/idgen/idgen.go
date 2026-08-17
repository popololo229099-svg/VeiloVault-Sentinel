package idgen

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
	"sync"
	"time"
)

type IDGenerator interface {
	Generate() string
	GenerateBatch(count int) []string
}

type UUIDv4Generator struct{}

func NewUUIDv4Generator() *UUIDv4Generator {
	return &UUIDv4Generator{}
}

func (g *UUIDv4Generator) Generate() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (g *UUIDv4Generator) GenerateBatch(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = g.Generate()
	}
	return ids
}

type UUIDv5Generator struct {
	namespace string
	name      string
	mu        sync.RWMutex
}

func NewUUIDv5Generator(namespace string) *UUIDv5Generator {
	return &UUIDv5Generator{namespace: namespace}
}

func (g *UUIDv5Generator) Generate() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	input := g.namespace + g.name
	b := make([]byte, 16)
	copy(b, []byte(input))

	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (g *UUIDv5Generator) GenerateWith(name string) string {
	input := g.namespace + name
	b := make([]byte, 16)
	copy(b, []byte(input))

	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (g *UUIDv5Generator) GenerateBatch(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = g.Generate()
	}
	return ids
}

func (g *UUIDv5Generator) SetName(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.name = name
}

type SnowflakeGenerator struct {
	machineID int64
	sequence  int64
	lastTime  int64
	mu        sync.Mutex
}

func NewSnowflakeGenerator(machineID int64) *SnowflakeGenerator {
	return &SnowflakeGenerator{
		machineID: machineID & 0x3FF,
	}
}

func (g *SnowflakeGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now().UnixMilli()

	if now == g.lastTime {
		g.sequence = (g.sequence + 1) & 0xFFF
		if g.sequence == 0 {
			for now <= g.lastTime {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		g.sequence = 0
	}

	g.lastTime = now

	id := ((now - 1288834974657) << 22) | (g.machineID << 12) | g.sequence
	return fmt.Sprintf("%d", id)
}

func (g *SnowflakeGenerator) GenerateBatch(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = g.Generate()
	}
	return ids
}

func (g *SnowflakeGenerator) Parse(id string) (timestamp int64, machineID int64, sequence int64) {
	var num int64
	fmt.Sscanf(id, "%d", &num)

	sequence = num & 0xFFF
	machineID = (num >> 12) & 0x3FF
	timestamp = (num >> 22) + 1288834974657
	return
}

type ULIDGenerator struct {
	mu sync.Mutex
}

func NewULIDGenerator() *ULIDGenerator {
	return &ULIDGenerator{}
}

func (g *ULIDGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	t := time.Now().UnixMilli()
	b := make([]byte, 10)
	rand.Read(b)

	ulidBytes := make([]byte, 16)
	binary.BigEndian.PutUint64(ulidBytes[:8], uint64(t))
	copy(ulidBytes[8:], b[:8])

	return base64.RawURLEncoding.EncodeToString(ulidBytes)
}

func (g *ULIDGenerator) GenerateBatch(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = g.Generate()
	}
	return ids
}

type KSUIDGenerator struct {
	mu sync.Mutex
}

func NewKSUIDGenerator() *KSUIDGenerator {
	return &KSUIDGenerator{}
}

func (g *KSUIDGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	t := time.Now().Unix() - 1400000000
	b := make([]byte, 20)
	rand.Read(b)

	binary.BigEndian.PutUint32(b[:4], uint32(t))

	return base64.RawURLEncoding.EncodeToString(b)
}

func (g *KSUIDGenerator) GenerateBatch(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = g.Generate()
	}
	return ids
}

type ShortIDGenerator struct {
	alphabet string
	length   int
	counter  uint64
	mu       sync.Mutex
}

func NewShortIDGenerator(length int) *ShortIDGenerator {
	if length <= 0 {
		length = 8
	}
	return &ShortIDGenerator{
		alphabet: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789",
		length:   length,
	}
}

func (g *ShortIDGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.counter++
	b := make([]byte, 16)
	rand.Read(b)

	result := make([]byte, g.length)
	for i := range result {
		result[i] = g.alphabet[int(b[i%len(b)])%len(g.alphabet)]
	}

	return string(result)
}

func (g *ShortIDGenerator) GenerateBatch(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = g.Generate()
	}
	return ids
}

func (g *ShortIDGenerator) SetAlphabet(alphabet string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.alphabet = alphabet
}

func (g *ShortIDGenerator) SetLength(length int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.length = length
}

type SequentialIDGenerator struct {
	current uint64
	prefix  string
	mu      sync.Mutex
}

func NewSequentialIDGenerator(prefix string) *SequentialIDGenerator {
	return &SequentialIDGenerator{
		current: 0,
		prefix:  prefix,
	}
}

func (g *SequentialIDGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.current++
	if g.prefix != "" {
		return fmt.Sprintf("%s_%010d", g.prefix, g.current)
	}
	return fmt.Sprintf("%010d", g.current)
}

func (g *SequentialIDGenerator) GenerateBatch(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = g.Generate()
	}
	return ids
}

func (g *SequentialIDGenerator) Current() uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.current
}

func (g *SequentialIDGenerator) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.current = 0
}

type DistributedIDGenerator struct {
	generator   IDGenerator
	namespace   string
	machineID   int64
	mu          sync.RWMutex
}

func NewDistributedIDGenerator(generator IDGenerator, namespace string, machineID int64) *DistributedIDGenerator {
	return &DistributedIDGenerator{
		generator: generator,
		namespace: namespace,
		machineID: machineID,
	}
}

func (g *DistributedIDGenerator) Generate() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	id := g.generator.Generate()
	if g.namespace != "" {
		return g.namespace + "_" + id
	}
	return id
}

func (g *DistributedIDGenerator) GenerateBatch(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = g.Generate()
	}
	return ids
}

func (g *DistributedIDGenerator) SetNamespace(namespace string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.namespace = namespace
}

type MonotonicIDGenerator struct {
	lastID    uint64
	randBytes []byte
	mu        sync.Mutex
}

func NewMonotonicIDGenerator() *MonotonicIDGenerator {
	return &MonotonicIDGenerator{
		lastID: 0,
	}
}

func (g *MonotonicIDGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.lastID++

	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, g.lastID)

	return fmt.Sprintf("%016x", g.lastID)
}

func (g *MonotonicIDGenerator) GenerateBatch(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = g.Generate()
	}
	return ids
}

type TimestampIDGenerator struct {
	sequence uint64
	mu       sync.Mutex
}

func NewTimestampIDGenerator() *TimestampIDGenerator {
	return &TimestampIDGenerator{}
}

func (g *TimestampIDGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.sequence++
	t := time.Now().UnixNano()
	return fmt.Sprintf("%x_%04d", t, g.sequence%10000)
}

func (g *TimestampIDGenerator) GenerateBatch(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = g.Generate()
	}
	return ids
}

type IDRegistry struct {
	generators map[string]IDGenerator
	mu         sync.RWMutex
}

func NewIDRegistry() *IDRegistry {
	return &IDRegistry{
		generators: make(map[string]IDGenerator),
	}
}

func (r *IDRegistry) Register(name string, gen IDGenerator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.generators[name] = gen
}

func (r *IDRegistry) Get(name string) IDGenerator {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.generators[name]
}

func (r *IDRegistry) Generate(name string) (string, error) {
	gen := r.Get(name)
	if gen == nil {
		return "", fmt.Errorf("generator not found: %s", name)
	}
	return gen.Generate(), nil
}

func (r *IDRegistry) ListGenerators() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.generators))
	for name := range r.generators {
		names = append(names, name)
	}
	return names
}

func (r *IDRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.generators, name)
}

type ComposedIDGenerator struct {
	generators []IDGenerator
	separator  string
	mu         sync.RWMutex
}

func NewComposedIDGenerator(separator string, generators ...IDGenerator) *ComposedIDGenerator {
	return &ComposedIDGenerator{
		generators: generators,
		separator:  separator,
	}
}

func (g *ComposedIDGenerator) Generate() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	parts := make([]string, len(g.generators))
	for i, gen := range g.generators {
		parts[i] = gen.Generate()
	}
	return strings.Join(parts, g.separator)
}

func (g *ComposedIDGenerator) GenerateBatch(count int) []string {
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = g.Generate()
	}
	return ids
}

func (g *ComposedIDGenerator) AddGenerator(gen IDGenerator) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.generators = append(g.generators, gen)
}
