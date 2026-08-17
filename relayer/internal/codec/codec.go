package codec

import (
	"encoding/json"
	"fmt"
	"sync"
)

type Encoder interface {
	Encode(v interface{}) ([]byte, error)
	Name() string
}

type Decoder interface {
	Decode(data []byte, v interface{}) error
	Name() string
}

type Codec interface {
	Encoder
	Decoder
}

type JSONCodec struct {
	prettyPrint bool
	mu          sync.RWMutex
}

func NewJSONCodec(pretty ...bool) *JSONCodec {
	pp := false
	if len(pretty) > 0 {
		pp = pretty[0]
	}
	return &JSONCodec{prettyPrint: pp}
}

func (jc *JSONCodec) Encode(v interface{}) ([]byte, error) {
	jc.mu.RLock()
	defer jc.mu.RUnlock()
	if jc.prettyPrint {
		return json.MarshalIndent(v, "", "  ")
	}
	return json.Marshal(v)
}

func (jc *JSONCodec) Decode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (jc *JSONCodec) Name() string { return "json" }

type BinaryCodec struct {
	mu sync.RWMutex
}

func NewBinaryCodec() *BinaryCodec {
	return &BinaryCodec{}
}

func (bc *BinaryCodec) Encode(v interface{}) ([]byte, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	switch val := v.(type) {
	case []byte:
		result := make([]byte, len(val))
		copy(result, val)
		return result, nil
	case string:
		return []byte(val), nil
	default:
		return json.Marshal(v)
	}
}

func (bc *BinaryCodec) Decode(data []byte, v interface{}) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	switch ptr := v.(type) {
	case *[]byte:
		*ptr = make([]byte, len(data))
		copy(*ptr, data)
		return nil
	case *string:
		*ptr = string(data)
		return nil
	default:
		return json.Unmarshal(data, v)
	}
}

func (bc *BinaryCodec) Name() string { return "binary" }

type GobCodec struct {
	mu sync.RWMutex
}

func NewGobCodec() *GobCodec {
	return &GobCodec{}
}

func (gc *GobCodec) Encode(v interface{}) ([]byte, error) {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return json.Marshal(v)
}

func (gc *GobCodec) Decode(data []byte, v interface{}) error {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return json.Unmarshal(data, v)
}

func (gc *GobCodec) Name() string { return "gob" }

type MessagePackCodec struct {
	mu sync.RWMutex
}

func NewMessagePackCodec() *MessagePackCodec {
	return &MessagePackCodec{}
}

func (mc *MessagePackCodec) Encode(v interface{}) ([]byte, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return json.Marshal(v)
}

func (mc *MessagePackCodec) Decode(data []byte, v interface{}) error {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return json.Unmarshal(data, v)
}

func (mc *MessagePackCodec) Name() string { return "msgpack" }

type CodecRegistry struct {
	codecs map[string]Codec
	mu     sync.RWMutex
}

func NewCodecRegistry() *CodecRegistry {
	return &CodecRegistry{
		codecs: make(map[string]Codec),
	}
}

func (cr *CodecRegistry) Register(codec Codec) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.codecs[codec.Name()] = codec
}

func (cr *CodecRegistry) Get(name string) Codec {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	return cr.codecs[name]
}

func (cr *CodecRegistry) List() []string {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	names := make([]string, 0, len(cr.codecs))
	for name := range cr.codecs {
		names = append(names, name)
	}
	return names
}

func (cr *CodecRegistry) MustEncode(name string, v interface{}) ([]byte, error) {
	codec := cr.Get(name)
	if codec == nil {
		return nil, fmt.Errorf("codec not found: %s", name)
	}
	return codec.Encode(v)
}

func (cr *CodecRegistry) MustDecode(name string, data []byte, v interface{}) error {
	codec := cr.Get(name)
	if codec == nil {
		return fmt.Errorf("codec not found: %s", name)
	}
	return codec.Decode(data, v)
}

type AutoDetectCodec struct {
	codecs *CodecRegistry
	mu     sync.RWMutex
}

func NewAutoDetectCodec() *AutoDetectCodec {
	registry := NewCodecRegistry()
	registry.Register(NewJSONCodec())
	registry.Register(NewBinaryCodec())

	return &AutoDetectCodec{
		codecs: registry,
	}
}

func (adc *AutoDetectCodec) Detect(data []byte) string {
	if len(data) == 0 {
		return "json"
	}
	if data[0] == '{' || data[0] == '[' {
		return "json"
	}
	return "binary"
}

func (adc *AutoDetectCodec) Encode(v interface{}) ([]byte, error) {
	codec := adc.codecs.Get("json")
	return codec.Encode(v)
}

func (adc *AutoDetectCodec) Decode(data []byte, v interface{}) error {
	format := adc.Detect(data)
	codec := adc.codecs.Get(format)
	if codec == nil {
		codec = adc.codecs.Get("json")
	}
	return codec.Decode(data, v)
}

func (adc *AutoDetectCodec) Name() string { return "auto" }

func (adc *AutoDetectCodec) RegisterCodec(codec Codec) {
	adc.codecs.Register(codec)
}

type EncoderDecoderChain struct {
	encoders []Encoder
	decoders []Decoder
	mu       sync.RWMutex
}

func NewEncoderDecoderChain() *EncoderDecoderChain {
	return &EncoderDecoderChain{
		encoders: make([]Encoder, 0),
		decoders: make([]Decoder, 0),
	}
}

func (edc *EncoderDecoderChain) AddEncoder(encoder Encoder) {
	edc.mu.Lock()
	defer edc.mu.Unlock()
	edc.encoders = append(edc.encoders, encoder)
}

func (edc *EncoderDecoderChain) AddDecoder(decoder Decoder) {
	edc.mu.Lock()
	defer edc.mu.Unlock()
	edc.decoders = append(edc.decoders, decoder)
}

func (edc *EncoderDecoderChain) Encode(v interface{}) ([]byte, error) {
	edc.mu.RLock()
	defer edc.mu.RUnlock()

	if len(edc.encoders) == 0 {
		return nil, fmt.Errorf("no encoders configured")
	}

	data, err := edc.encoders[0].Encode(v)
	if err != nil {
		return nil, err
	}

	for _, enc := range edc.encoders[1:] {
		data, err = enc.Encode(data)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

func (edc *EncoderDecoderChain) Decode(data []byte, v interface{}) error {
	edc.mu.RLock()
	defer edc.mu.RUnlock()

	if len(edc.decoders) == 0 {
		return fmt.Errorf("no decoders configured")
	}

	current := data
	var err error
	for i := len(edc.decoders) - 1; i >= 0; i-- {
		err = edc.decoders[i].Decode(current, &current)
		if err != nil {
			return err
		}
	}

	return edc.decoders[0].Decode(current, v)
}

func (edc *EncoderDecoderChain) Name() string { return "chain" }

type TypedEncoder[T any] struct {
	codec  Codec
	name   string
	mu     sync.RWMutex
}

func NewTypedEncoder[T any](codec Codec, name string) *TypedEncoder[T] {
	return &TypedEncoder[T]{codec: codec, name: name}
}

func (te *TypedEncoder[T]) Encode(v T) ([]byte, error) {
	te.mu.RLock()
	defer te.mu.RUnlock()
	return te.codec.Encode(v)
}

func (te *TypedEncoder[T]) Decode(data []byte) (T, error) {
	te.mu.RLock()
	defer te.mu.RUnlock()
	var result T
	err := te.codec.Decode(data, &result)
	return result, err
}

func (te *TypedEncoder[T]) Name() string {
	return te.name
}

type ValidationCodec struct {
	inner    Codec
	validate func(data []byte) error
	mu       sync.RWMutex
}

func NewValidationCodec(inner Codec, validate func([]byte) error) *ValidationCodec {
	return &ValidationCodec{
		inner:    inner,
		validate: validate,
	}
}

func (vc *ValidationCodec) Encode(v interface{}) ([]byte, error) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.inner.Encode(v)
}

func (vc *ValidationCodec) Decode(data []byte, v interface{}) error {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	if vc.validate != nil {
		if err := vc.validate(data); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	return vc.inner.Decode(data, v)
}

func (vc *ValidationCodec) Name() string { return vc.inner.Name() }
