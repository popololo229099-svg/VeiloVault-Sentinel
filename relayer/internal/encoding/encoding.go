package encoding

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type Codec interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
	Name() string
}

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

type JSONCodec struct {
	prettyPrint bool
	mu          sync.RWMutex
}

func NewJSONCodec(prettyPrint ...bool) *JSONCodec {
	pp := false
	if len(prettyPrint) > 0 {
		pp = prettyPrint[0]
	}
	return &JSONCodec{prettyPrint: pp}
}

func (jc *JSONCodec) Marshal(v interface{}) ([]byte, error) {
	jc.mu.RLock()
	defer jc.mu.RUnlock()
	if jc.prettyPrint {
		return json.MarshalIndent(v, "", "  ")
	}
	return json.Marshal(v)
}

func (jc *JSONCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (jc *JSONCodec) Name() string { return "json" }

type BinaryCodec struct {
	mu sync.RWMutex
}

func NewBinaryCodec() *BinaryCodec {
	return &BinaryCodec{}
}

func (bc *BinaryCodec) Marshal(v interface{}) ([]byte, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	switch val := v.(type) {
	case []byte:
		return val, nil
	case string:
		return []byte(val), nil
	case int:
		return []byte(fmt.Sprintf("%d", val)), nil
	default:
		return []byte(fmt.Sprintf("%v", val)), nil
	}
}

func (bc *BinaryCodec) Unmarshal(data []byte, v interface{}) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	switch ptr := v.(type) {
	case *[]byte:
		*ptr = data
	case *string:
		*ptr = string(data)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
	return nil
}

func (bc *BinaryCodec) Name() string { return "binary" }

type Base64Codec struct {
	encoding *base64.Encoding
	mu       sync.RWMutex
}

func NewBase64Codec(variant ...string) *Base64Codec {
	enc := base64.StdEncoding
	if len(variant) > 0 {
		switch variant[0] {
		case "url":
			enc = base64.URLEncoding
		case "raw":
			enc = base64.RawStdEncoding
		case "rawurl":
			enc = base64.RawURLEncoding
		}
	}
	return &Base64Codec{encoding: enc}
}

func (b64 *Base64Codec) Marshal(v interface{}) ([]byte, error) {
	b64.mu.RLock()
	defer b64.mu.RUnlock()

	var data []byte
	switch val := v.(type) {
	case []byte:
		data = val
	case string:
		data = []byte(val)
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}

	encoded := make([]byte, b64.encoding.EncodedLen(len(data)))
	b64.encoding.Encode(encoded, data)
	return encoded, nil
}

func (b64 *Base64Codec) Unmarshal(data []byte, v interface{}) error {
	b64.mu.RLock()
	defer b64.mu.RUnlock()

	decoded, err := b64.encoding.DecodeString(string(data))
	if err != nil {
		return err
	}

	switch ptr := v.(type) {
	case *[]byte:
		*ptr = decoded
	case *string:
		*ptr = string(decoded)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
	return nil
}

func (b64 *Base64Codec) Name() string { return "base64" }

type HexCodec struct {
	mu sync.RWMutex
}

func NewHexCodec() *HexCodec {
	return &HexCodec{}
}

func (hc *HexCodec) Marshal(v interface{}) ([]byte, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	var data []byte
	switch val := v.(type) {
	case []byte:
		data = val
	case string:
		data = []byte(val)
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}

	encoded := make([]byte, hex.EncodedLen(len(data)))
	hex.Encode(encoded, data)
	return encoded, nil
}

func (hc *HexCodec) Unmarshal(data []byte, v interface{}) error {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	decoded, err := hex.DecodeString(string(data))
	if err != nil {
		return err
	}

	switch ptr := v.(type) {
	case *[]byte:
		*ptr = decoded
	case *string:
		*ptr = string(decoded)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
	return nil
}

func (hc *HexCodec) Name() string { return "hex" }

type Base58Codec struct {
	alphabet string
	mu       sync.RWMutex
}

var base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func NewBase58Codec() *Base58Codec {
	return &Base58Codec{alphabet: base58Alphabet}
}

func (b58 *Base58Codec) Marshal(v interface{}) ([]byte, error) {
	b58.mu.RLock()
	defer b58.mu.RUnlock()

	var data []byte
	switch val := v.(type) {
	case []byte:
		data = val
	case string:
		data = []byte(val)
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}

	result := b58.encode(data)
	return []byte(result), nil
}

func (b58 *Base58Codec) Unmarshal(data []byte, v interface{}) error {
	b58.mu.RLock()
	defer b58.mu.RUnlock()

	decoded, err := b58.decode(string(data))
	if err != nil {
		return err
	}

	switch ptr := v.(type) {
	case *[]byte:
		*ptr = decoded
	case *string:
		*ptr = string(decoded)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
	return nil
}

func (b58 *Base58Codec) encode(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	num := make([]int, len(data))
	for i, b := range data {
		num[i] = int(b)
	}

	result := make([]byte, 0)
	for {
		remainder := 0
		newNum := make([]int, 0)
		for _, n := range num {
			acc := remainder*256 + n
			remainder = acc % 58
			if len(newNum) > 0 || acc/58 > 0 {
				newNum = append(newNum, acc/58)
			}
		}
		num = newNum
		result = append(result, b58.alphabet[remainder])

		if len(num) == 0 {
			break
		}
	}

	for i, b := range data {
		if b == 0 && i < len(data)-1 {
			result = append(result, b58.alphabet[0])
		} else {
			break
		}
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

func (b58 *Base58Codec) decode(s string) ([]byte, error) {
	if len(s) == 0 {
		return nil, nil
	}

	result := []byte{0}
	for _, c := range s {
		index := strings.IndexRune(b58.alphabet, c)
		if index < 0 {
			return nil, fmt.Errorf("invalid base58 character: %c", c)
		}

		acc := 0
		for i, b := range result {
			acc += int(b) * 58
			result[i] = byte(acc % 256)
			acc /= 256
		}
		for acc > 0 {
			result = append(result, byte(acc%256))
			acc /= 256
		}
	}

	for _, c := range s {
		if rune(b58.alphabet[0]) == c {
			result = append([]byte{0}, result...)
		} else {
			break
		}
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

func (b58 *Base58Codec) Name() string { return "base58" }

type VarintCodec struct {
	mu sync.RWMutex
}

func NewVarintCodec() *VarintCodec {
	return &VarintCodec{}
}

func (vc *VarintCodec) Marshal(v interface{}) ([]byte, error) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	var n uint64
	switch val := v.(type) {
	case int:
		n = uint64(val)
	case int64:
		n = uint64(val)
	case uint:
		n = uint64(val)
	case uint64:
		n = val
	default:
		return nil, fmt.Errorf("unsupported type: %T", v)
	}

	result := vc.encodeVarint(n)
	return result, nil
}

func (vc *VarintCodec) Unmarshal(data []byte, v interface{}) error {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	n, _ := vc.decodeVarint(data)

	switch ptr := v.(type) {
	case *int:
		*ptr = int(n)
	case *int64:
		*ptr = int64(n)
	case *uint:
		*ptr = uint(n)
	case *uint64:
		*ptr = n
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
	return nil
}

func (vc *VarintCodec) encodeVarint(n uint64) []byte {
	result := make([]byte, 0, 10)
	for n >= 0x80 {
		result = append(result, byte(n)|0x80)
		n >>= 7
	}
	result = append(result, byte(n))
	return result
}

func (vc *VarintCodec) decodeVarint(data []byte) (uint64, int) {
	var result uint64
	var shift uint
	for i, b := range data {
		result |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return result, i + 1
		}
		shift += 7
	}
	return result, len(data)
}

func (vc *VarintCodec) Name() string { return "varint" }

type AutoCodec struct {
	detect func([]byte) string
	codecs *CodecRegistry
	mu     sync.RWMutex
}

func NewAutoCodec() *AutoCodec {
	registry := NewCodecRegistry()
	registry.Register(NewJSONCodec())
	registry.Register(NewBase64Codec())
	registry.Register(NewHexCodec())

	return &AutoCodec{
		codecs: registry,
		detect: func(data []byte) string {
			if len(data) == 0 {
				return "json"
			}
			if data[0] == '{' || data[0] == '[' {
				return "json"
			}
			for _, b := range data {
				if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '+' || b == '/' || b == '=' {
					continue
				}
				return "binary"
			}
			return "base64"
		},
	}
}

func (ac *AutoCodec) Detect(data []byte) string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.detect(data)
}

func (ac *AutoCodec) Marshal(v interface{}) ([]byte, error) {
	codec := ac.codecs.Get("json")
	return codec.Marshal(v)
}

func (ac *AutoCodec) Unmarshal(data []byte, v interface{}) error {
	format := ac.Detect(data)
	codec := ac.codecs.Get(format)
	if codec == nil {
		codec = ac.codecs.Get("json")
	}
	return codec.Unmarshal(data, v)
}

func (ac *AutoCodec) Name() string { return "auto" }

type EncoderDecoder interface {
	Encode(v interface{}) ([]byte, error)
	Decode(data []byte, v interface{}) error
}

type CompositeCodec struct {
	primary    Codec
	fallbacks  []Codec
	mu         sync.RWMutex
}

func NewCompositeCodec(primary Codec, fallbacks ...Codec) *CompositeCodec {
	return &CompositeCodec{
		primary:   primary,
		fallbacks: fallbacks,
	}
}

func (cc *CompositeCodec) Marshal(v interface{}) ([]byte, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.primary.Marshal(v)
}

func (cc *CompositeCodec) Unmarshal(data []byte, v interface{}) error {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	if err := cc.primary.Unmarshal(data, v); err == nil {
		return nil
	}

	for _, fallback := range cc.fallbacks {
		if err := fallback.Unmarshal(data, v); err == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to unmarshal with any codec")
}

func (cc *CompositeCodec) Name() string { return "composite" }
