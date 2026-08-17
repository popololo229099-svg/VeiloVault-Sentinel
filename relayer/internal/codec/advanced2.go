package codec

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

type TimeCodec struct {
	format string
	mu     sync.RWMutex
}

func NewTimeCodec(format string) *TimeCodec {
	if format == "" {
		format = time.RFC3339
	}
	return &TimeCodec{format: format}
}

func (tc *TimeCodec) Encode(t time.Time) ([]byte, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return []byte(t.Format(tc.format)), nil
}

func (tc *TimeCodec) Decode(data []byte) (time.Time, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return time.Parse(tc.format, string(data))
}

func (tc *TimeCodec) Name() string { return "time" }

type IPCodec struct {
	isIPv6 bool
	mu     sync.RWMutex
}

func NewIPCodec(isIPv6 bool) *IPCodec {
	return &IPCodec{isIPv6: isIPv6}
}

func (ipc *IPCodec) Encode(ip string) ([]byte, error) {
	return []byte(ip), nil
}

func (ipc *IPCodec) Decode(data []byte) (string, error) {
	return string(data), nil
}

func (ipc *IPCodec) Name() string { return "ip" }

type StructCodec struct {
	fieldTag string
	mu       sync.RWMutex
}

func NewStructCodec(fieldTag string) *StructCodec {
	if fieldTag == "" {
		fieldTag = "codec"
	}
	return &StructCodec{fieldTag: fieldTag}
}

func (sc *StructCodec) Encode(v interface{}) ([]byte, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return json.Marshal(v)
}

func (sc *StructCodec) Decode(data []byte, v interface{}) error {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return json.Unmarshal(data, v)
}

func (sc *StructCodec) Name() string { return "struct" }

type BitfieldCodec struct {
	bits    uint
	mu      sync.RWMutex
}

func NewBitfieldCodec(bits uint) *BitfieldCodec {
	if bits == 0 {
		bits = 8
	}
	return &BitfieldCodec{bits: bits}
}

func (bfc *BitfieldCodec) Encode(flags []bool) []byte {
	bfc.mu.RLock()
	defer bfc.mu.RUnlock()

	byteLen := (len(flags) + 7) / 8
	result := make([]byte, byteLen)

	for i, flag := range flags {
		if flag {
			result[i/8] |= 1 << uint(i%8)
		}
	}

	return result
}

func (bfc *BitfieldCodec) Decode(data []byte, length int) []bool {
	bfc.mu.RLock()
	defer bfc.mu.RUnlock()

	result := make([]bool, length)
	for i := 0; i < length && i < len(data)*8; i++ {
		result[i] = (data[i/8]>>uint(i%8))&1 == 1
	}
	return result
}

func (bfc *BitfieldCodec) Name() string { return "bitfield" }

type EnumCodec struct {
	values map[string]int
	names  map[int]string
	mu     sync.RWMutex
}

func NewEnumCodec(values map[string]int) *EnumCodec {
	names := make(map[int]string, len(values))
	for name, val := range values {
		names[val] = name
	}
	return &EnumCodec{values: values, names: names}
}

func (ec *EnumCodec) Encode(name string) ([]byte, error) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	val, ok := ec.values[name]
	if !ok {
		return nil, fmt.Errorf("unknown enum value: %s", name)
	}
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(val))
	return buf, nil
}

func (ec *EnumCodec) Decode(data []byte) (string, error) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	if len(data) < 4 {
		return "", fmt.Errorf("data too short")
	}
	val := binary.LittleEndian.Uint32(data)
	name, ok := ec.names[int(val)]
	if !ok {
		return "", fmt.Errorf("unknown enum code: %d", val)
	}
	return name, nil
}

func (ec *EnumCodec) Name() string { return "enum" }

type Float16Codec struct {
	mu sync.RWMutex
}

func NewFloat16Codec() *Float16Codec {
	return &Float16Codec{}
}

func (f16 *Float16Codec) Encode(v float32) uint16 {
	f16.mu.RLock()
	defer f16.mu.RUnlock()
	f := float64(v)
	bits := math.Float64bits(f)
	sign := uint16((bits>>48)&0x8000)
	exponent := int((bits>>52)&0x7FF) - 1023 + 15
	mantissa := uint16((bits>>42) & 0x3FF)

	if exponent <= 0 {
		return sign
	}
	if exponent >= 31 {
		return sign | 0x7C00
	}
	return sign | uint16(exponent<<10) | mantissa
}

func (f16 *Float16Codec) Decode(v uint16) float32 {
	f16.mu.RLock()
	defer f16.mu.RUnlock()
	sign := uint64(v&0x8000) << 48
	exponent := int((v>>10)&0x1F) - 15 + 1023
	mantissa := uint64(v&0x3FF) << 42

	if exponent <= 0 {
		return float32(math.Float64frombits(sign))
	}

	bits := sign | uint64(exponent<<52) | mantissa
	return float32(math.Float64frombits(bits))
}

func (f16 *Float16Codec) Name() string { return "float16" }

type ZigZagCodec struct {
	mu sync.RWMutex
}

func NewZigZagCodec() *ZigZagCodec {
	return &ZigZagCodec{}
}

func (zzc *ZigZagCodec) Encode(n int64) uint64 {
	zzc.mu.RLock()
	defer zzc.mu.RUnlock()
	return uint64((n << 1) ^ (n >> 63))
}

func (zzc *ZigZagCodec) Decode(n uint64) int64 {
	zzc.mu.RLock()
	defer zzc.mu.RUnlock()
	return int64((n >> 1) ^ -(n & 1))
}

func (zzc *ZigZagCodec) Name() string { return "zigzag" }

type DecimalCodec struct {
	precision int
	scale     int
	mu        sync.RWMutex
}

func NewDecimalCodec(precision, scale int) *DecimalCodec {
	if precision <= 0 {
		precision = 10
	}
	if scale < 0 {
		scale = 2
	}
	return &DecimalCodec{precision: precision, scale: scale}
}

func (dc *DecimalCodec) Encode(value float64) []byte {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	scaled := value * math.Pow10(dc.scale)
	return []byte(fmt.Sprintf("%.0f", scaled))
}

func (dc *DecimalCodec) Decode(data []byte) (float64, error) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	var scaled float64
	_, err := fmt.Sscanf(string(data), "%f", &scaled)
	if err != nil {
		return 0, err
	}
	return scaled / math.Pow10(dc.scale), nil
}

func (dc *DecimalCodec) Name() string { return "decimal" }

type PackedCodec struct {
	mu sync.RWMutex
}

func NewPackedCodec() *PackedCodec {
	return &PackedCodec{}
}

func (pc *PackedCodec) EncodeBool(b bool) byte {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	if b {
		return 1
	}
	return 0
}

func (pc *PackedCodec) DecodeBool(b byte) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return b != 0
}

func (pc *PackedCodec) EncodeUint16(v uint16) []byte {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return []byte{byte(v), byte(v >> 8)}
}

func (pc *PackedCodec) DecodeUint16(data []byte) uint16 {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	if len(data) < 2 {
		return 0
	}
	return uint16(data[0]) | uint16(data[1])<<8
}

func (pc *PackedCodec) EncodeUint32(v uint32) []byte {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

func (pc *PackedCodec) DecodeUint32(data []byte) uint32 {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	if len(data) < 4 {
		return 0
	}
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
}

func (pc *PackedCodec) EncodeUint64(v uint64) []byte {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	data := make([]byte, 8)
	for i := 0; i < 8; i++ {
		data[i] = byte(v >> uint(i*8))
	}
	return data
}

func (pc *PackedCodec) DecodeUint64(data []byte) uint64 {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	if len(data) < 8 {
		return 0
	}
	var v uint64
	for i := 0; i < 8; i++ {
		v |= uint64(data[i]) << uint(i*8)
	}
	return v
}

func (pc *PackedCodec) Name() string { return "packed" }

type TLVCodec struct {
	mu sync.RWMutex
}

type TLV struct {
	Type    byte
	Length  uint16
	Value   []byte
}

func NewTLVCodec() *TLVCodec {
	return &TLVCodec{}
}

func (tlv *TLVCodec) Encode(tlvItem TLV) []byte {
	tlv.mu.RLock()
	defer tlv.mu.RUnlock()

	data := make([]byte, 3+len(tlvItem.Value))
	data[0] = tlvItem.Type
	data[1] = byte(tlvItem.Length >> 8)
	data[2] = byte(tlvItem.Length)
	copy(data[3:], tlvItem.Value)
	return data
}

func (tlv *TLVCodec) Decode(data []byte) (TLV, int, error) {
	tlv.mu.RLock()
	defer tlv.mu.RUnlock()

	if len(data) < 3 {
		return TLV{}, 0, fmt.Errorf("data too short")
	}

	t := data[0]
	length := uint16(data[1])<<8 | uint16(data[2])

	if int(length)+3 > len(data) {
		return TLV{}, 0, fmt.Errorf("insufficient data for length %d", length)
	}

	return TLV{
		Type:   t,
		Length: length,
		Value:  data[3 : 3+length],
	}, 3 + int(length), nil
}

func (tlv *TLVCodec) Name() string { return "tlv" }
