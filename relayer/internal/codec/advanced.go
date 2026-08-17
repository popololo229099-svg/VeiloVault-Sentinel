package codec

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
)

type StringCodec struct {
	delimiter string
	mu        sync.RWMutex
}

func NewStringCodec(delimiter string) *StringCodec {
	if delimiter == "" {
		delimiter = ","
	}
	return &StringCodec{delimiter: delimiter}
}

func (sc *StringCodec) Encode(v interface{}) ([]byte, error) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	switch val := v.(type) {
	case []string:
		return []byte(strings.Join(val, sc.delimiter)), nil
	case string:
		return []byte(val), nil
	case []byte:
		return val, nil
	default:
		return []byte(fmt.Sprintf("%v", v)), nil
	}
}

func (sc *StringCodec) Decode(data []byte, v interface{}) error {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	switch ptr := v.(type) {
	case *[]string:
		*ptr = strings.Split(string(data), sc.delimiter)
		return nil
	case *string:
		*ptr = string(data)
		return nil
	case *[]byte:
		*ptr = make([]byte, len(data))
		copy(*ptr, data)
		return nil
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

func (sc *StringCodec) Name() string { return "string" }

type NumberCodec struct {
	mu sync.RWMutex
}

func NewNumberCodec() *NumberCodec {
	return &NumberCodec{}
}

func (nc *NumberCodec) EncodeInt(v int) []byte {
	return []byte(strconv.Itoa(v))
}

func (nc *NumberCodec) DecodeInt(data []byte) (int, error) {
	return strconv.Atoi(string(data))
}

func (nc *NumberCodec) EncodeFloat(v float64) []byte {
	return []byte(strconv.FormatFloat(v, 'f', -1, 64))
}

func (nc *NumberCodec) DecodeFloat(data []byte) (float64, error) {
	return strconv.ParseFloat(string(data), 64)
}

func (nc *NumberCodec) EncodeBool(v bool) []byte {
	return []byte(strconv.FormatBool(v))
}

func (nc *NumberCodec) DecodeBool(data []byte) (bool, error) {
	return strconv.ParseBool(string(data))
}

func (nc *NumberCodec) Name() string { return "number" }

type HexCodec struct {
	upper bool
	mu    sync.RWMutex
}

func NewHexCodec(upper ...bool) *HexCodec {
	u := false
	if len(upper) > 0 {
		u = upper[0]
	}
	return &HexCodec{upper: u}
}

func (hc *HexCodec) Encode(data []byte) string {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	if hc.upper {
		return hex.EncodeToString(data)
	}
	return hex.EncodeToString(data)
}

func (hc *HexCodec) Decode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

func (hc *HexCodec) Name() string { return "hex" }

type Base58Codec struct {
	alphabet string
	mu       sync.RWMutex
}

func NewBase58Codec() *Base58Codec {
	return &Base58Codec{alphabet: "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"}
}

func (bc *Base58Codec) Encode(data []byte) string {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	num := 0
	for _, b := range data {
		num = num*256 + int(b)
	}

	result := make([]byte, 0)
	for num > 0 {
		result = append(result, bc.alphabet[num%58])
		num /= 58
	}

	for _, b := range data {
		if b == 0 {
			result = append(result, bc.alphabet[0])
		} else {
			break
		}
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}

func (bc *Base58Codec) Decode(s string) ([]byte, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	num := 0
	for _, c := range s {
		idx := strings.IndexRune(bc.alphabet, c)
		if idx < 0 {
			return nil, fmt.Errorf("invalid base58 character: %c", c)
		}
		num = num*58 + idx
	}

	result := make([]byte, 0)
	for num > 0 {
		result = append(result, byte(num%256))
		num /= 256
	}

	for _, c := range s {
		if byte(c) == bc.alphabet[0] {
			result = append(result, 0)
		} else {
			break
		}
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

func (bc *Base58Codec) Name() string { return "base58" }

type LengthPrefixCodec struct {
	byteOrder string
	mu        sync.RWMutex
}

func NewLengthPrefixCodec(byteOrder string) *LengthPrefixCodec {
	if byteOrder == "" {
		byteOrder = "big"
	}
	return &LengthPrefixCodec{byteOrder: byteOrder}
}

func (lp *LengthPrefixCodec) Encode(data []byte) []byte {
	lp.mu.RLock()
	defer lp.mu.RUnlock()

	length := len(data)
	prefix := make([]byte, 4)

	if lp.byteOrder == "big" {
		prefix[0] = byte(length >> 24)
		prefix[1] = byte(length >> 16)
		prefix[2] = byte(length >> 8)
		prefix[3] = byte(length)
	} else {
		prefix[0] = byte(length)
		prefix[1] = byte(length >> 8)
		prefix[2] = byte(length >> 16)
		prefix[3] = byte(length >> 24)
	}

	return append(prefix, data...)
}

func (lp *LengthPrefixCodec) Decode(data []byte) ([]byte, error) {
	lp.mu.RLock()
	defer lp.mu.RUnlock()

	if len(data) < 4 {
		return nil, fmt.Errorf("data too short")
	}

	var length int
	if lp.byteOrder == "big" {
		length = int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	} else {
		length = int(data[3])<<24 | int(data[2])<<16 | int(data[1])<<8 | int(data[0])
	}

	if length < 0 || length > len(data)-4 {
		return nil, fmt.Errorf("invalid length: %d", length)
	}

	return data[4 : 4+length], nil
}

func (lp *LengthPrefixCodec) Name() string { return "length_prefix" }

type TaggedCodec struct {
	tags map[string]Codec
	mu   sync.RWMutex
}

func NewTaggedCodec() *TaggedCodec {
	return &TaggedCodec{
		tags: make(map[string]Codec),
	}
}

func (tc *TaggedCodec) Register(tag string, codec Codec) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.tags[tag] = codec
}

func (tc *TaggedCodec) Encode(tag string, v interface{}) ([]byte, error) {
	tc.mu.RLock()
	codec, ok := tc.tags[tag]
	tc.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown tag: %s", tag)
	}
	return codec.Encode(v)
}

func (tc *TaggedCodec) Decode(tag string, data []byte, v interface{}) error {
	tc.mu.RLock()
	codec, ok := tc.tags[tag]
	tc.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown tag: %s", tag)
	}
	return codec.Decode(data, v)
}

func (tc *TaggedCodec) Name() string { return "tagged" }

type VersionedCodec struct {
	version  int
	codecs   map[int]Codec
	mu       sync.RWMutex
}

func NewVersionedCodec() *VersionedCodec {
	return &VersionedCodec{
		version: 1,
		codecs:  make(map[int]Codec),
	}
}

func (vc *VersionedCodec) Register(version int, codec Codec) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.codecs[version] = codec
}

func (vc *VersionedCodec) SetVersion(version int) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.version = version
}

func (vc *VersionedCodec) Encode(v interface{}) ([]byte, error) {
	vc.mu.RLock()
	codec, ok := vc.codecs[vc.version]
	version := vc.version
	vc.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown version: %d", version)
	}

	data, err := codec.Encode(v)
	if err != nil {
		return nil, err
	}

	header := []byte{byte(version >> 8), byte(version)}
	return append(header, data...), nil
}

func (vc *VersionedCodec) Decode(data []byte, v interface{}) error {
	if len(data) < 2 {
		return fmt.Errorf("data too short")
	}

	version := int(data[0])<<8 | int(data[1])

	vc.mu.RLock()
	codec, ok := vc.codecs[version]
	vc.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown version: %d", version)
	}

	return codec.Decode(data[2:], v)
}

func (vc *VersionedCodec) Name() string { return "versioned" }

type DeltaCodec struct {
	inner    Codec
	deltas   []interface{}
	mu       sync.RWMutex
}

func NewDeltaCodec(inner Codec) *DeltaCodec {
	return &DeltaCodec{
		inner:  inner,
		deltas: make([]interface{}, 0),
	}
}

func (dc *DeltaCodec) Encode(v interface{}) ([]byte, error) {
	return dc.inner.Encode(v)
}

func (dc *DeltaCodec) Decode(data []byte, v interface{}) error {
	return dc.inner.Decode(data, v)
}

func (dc *DeltaCodec) Name() string { return "delta_" + dc.inner.Name() }

type ChecksumCodec struct {
	inner    Codec
	checksum func([]byte) []byte
	mu       sync.RWMutex
}

func NewChecksumCodec(inner Codec, checksum func([]byte) []byte) *ChecksumCodec {
	if checksum == nil {
		checksum = func(data []byte) []byte {
			sum := uint32(0)
			for _, b := range data {
				sum += uint32(b)
			}
			return []byte{byte(sum >> 24), byte(sum >> 16), byte(sum >> 8), byte(sum)}
		}
	}
	return &ChecksumCodec{
		inner:    inner,
		checksum: checksum,
	}
}

func (cc *ChecksumCodec) Encode(v interface{}) ([]byte, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	data, err := cc.inner.Encode(v)
	if err != nil {
		return nil, err
	}

	cs := cc.checksum(data)
	return append(data, cs...), nil
}

func (cc *ChecksumCodec) Decode(data []byte, v interface{}) error {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	if len(data) < 4 {
		return fmt.Errorf("data too short")
	}

	payload := data[:len(data)-4]
	expectedCS := data[len(data)-4:]
	actualCS := cc.checksum(payload)

	for i := range expectedCS {
		if expectedCS[i] != actualCS[i] {
			return fmt.Errorf("checksum mismatch")
		}
	}

	return cc.inner.Decode(payload, v)
}

func (cc *ChecksumCodec) Name() string { return "checksum_" + cc.inner.Name() }

type VarintCodec struct {
	mu sync.RWMutex
}

func NewVarintCodec() *VarintCodec {
	return &VarintCodec{}
}

func (vc *VarintCodec) Encode(value int64) []byte {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	result := make([]byte, 0, 10)
	for value >= 0x80 {
		result = append(result, byte(value)|0x80)
		value >>= 7
	}
	result = append(result, byte(value))
	return result
}

func (vc *VarintCodec) Decode(data []byte) (int64, int) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	var result int64
	var shift uint
	for i, b := range data {
		result |= int64(b&0x7F) << shift
		if b < 0x80 {
			return result, i + 1
		}
		shift += 7
	}
	return 0, 0
}

func (vc *VarintCodec) Name() string { return "varint" }

type FixedWidthCodec struct {
	width int
	mu    sync.RWMutex
}

func NewFixedWidthCodec(width int) *FixedWidthCodec {
	if width <= 0 {
		width = 8
	}
	return &FixedWidthCodec{width: width}
}

func (fw *FixedWidthCodec) Encode(v float64) []byte {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	bits := math.Float64bits(v)
	result := make([]byte, 8)
	for i := 0; i < 8; i++ {
		result[i] = byte(bits >> uint(i*8))
	}
	return result
}

func (fw *FixedWidthCodec) Decode(data []byte) (float64, error) {
	fw.mu.RLock()
	defer fw.mu.RUnlock()

	if len(data) < 8 {
		return 0, fmt.Errorf("data too short")
	}

	var bits uint64
	for i := 0; i < 8; i++ {
		bits |= uint64(data[i]) << uint(i*8)
	}
	return math.Float64frombits(bits), nil
}

func (fw *FixedWidthCodec) Name() string { return "fixed_width" }

type EncodedPayload struct {
	Format  string
	Data    []byte
	Version int
}

type MultiFormatCodec struct {
	formats map[string]Codec
	mu      sync.RWMutex
}

func NewMultiFormatCodec() *MultiFormatCodec {
	return &MultiFormatCodec{
		formats: make(map[string]Codec),
	}
}

func (mf *MultiFormatCodec) Register(format string, codec Codec) {
	mf.mu.Lock()
	defer mf.mu.Unlock()
	mf.formats[format] = codec
}

func (mf *MultiFormatCodec) Encode(format string, v interface{}) (*EncodedPayload, error) {
	mf.mu.RLock()
	codec, ok := mf.formats[format]
	mf.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown format: %s", format)
	}

	data, err := codec.Encode(v)
	if err != nil {
		return nil, err
	}

	return &EncodedPayload{
		Format:  format,
		Data:    data,
		Version: 1,
	}, nil
}

func (mf *MultiFormatCodec) Decode(payload *EncodedPayload, v interface{}) error {
	mf.mu.RLock()
	codec, ok := mf.formats[payload.Format]
	mf.mu.RUnlock()

	if !ok {
		return fmt.Errorf("unknown format: %s", payload.Format)
	}

	return codec.Decode(payload.Data, v)
}

func (mf *MultiFormatCodec) Detect(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	if data[0] == '{' || data[0] == '[' {
		return "json"
	}
	_, err := base64.StdEncoding.DecodeString(string(data))
	if err == nil {
		return "base64"
	}
	return "binary"
}
