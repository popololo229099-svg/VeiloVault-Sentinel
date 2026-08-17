package encoder

import (
	"encoding/json"
	"fmt"
	"time"
)

// JSONEncoder provides JSON serialization for Solana types.
type JSONEncoder struct{}

func NewJSONEncoder() *JSONEncoder {
	return &JSONEncoder{}
}

func (je *JSONEncoder) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (je *JSONEncoder) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (je *JSONEncoder) MarshalInstructionData(discriminator [8]byte, fields ...interface{}) ([]byte, error) {
	result := make([]byte, 0, 8)
	result = append(result, discriminator[:]...)
	for _, field := range fields {
		switch v := field.(type) {
		case uint64:
			result = append(result, Uint64ToBytes(v)...)
		case uint32:
			result = append(result, Uint32ToBytes(v)...)
		case bool:
			result = append(result, BoolToByte(v))
		case []byte:
			result = append(result, Uint32ToBytes(uint32(len(v)))...)
			result = append(result, v...)
		case string:
			result = append(result, StringToBytes(v)...)
		}
	}
	return result, nil
}

// SlotEncoder encodes slot-specific data.
type SlotEncoder struct{}

func NewSlotEncoder() *SlotEncoder {
	return &SlotEncoder{}
}

func (se *SlotEncoder) EncodeSlot(slot uint64, parent uint64, status string) []byte {
	data := make([]byte, 0)
	data = append(data, Uint64ToBytes(slot)...)
	data = append(data, Uint64ToBytes(parent)...)
	data = append(data, StringToBytes(status)...)
	return data
}

func (se *SlotEncoder) DecodeSlot(data []byte) (slot, parent uint64, status string, err error) {
	if len(data) < 20 {
		err = fmt.Errorf("slot data too short")
		return
	}
	slot = BytesToUint64(data[:8])
	parent = BytesToUint64(data[8:16])
	statusLen := BytesToUint32(data[16:20])
	if len(data) < 20+int(statusLen) {
		err = fmt.Errorf("slot data too short for status")
		return
	}
	status = string(data[20 : 20+int(statusLen)])
	return
}

// TimestampEncoder encodes timestamps.
type TimestampEncoder struct{}

func NewTimestampEncoder() *TimestampEncoder {
	return &TimestampEncoder{}
}

func (te *TimestampEncoder) Encode(t time.Time) []byte {
	return Uint64ToBytes(uint64(t.UnixNano()))
}

func (te *TimestampEncoder) Decode(data []byte) (time.Time, error) {
	if len(data) < 8 {
		return time.Time{}, fmt.Errorf("timestamp data too short")
	}
	nanos := BytesToUint64(data[:8])
	return time.Unix(0, int64(nanos)), nil
}
