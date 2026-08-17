package encoder

import (
	"fmt"
	"time"
)

// AccountDecoder decodes Solana account data.
type AccountDecoder struct{}

func NewAccountDecoder() *AccountDecoder {
	return &AccountDecoder{}
}

func (d *AccountDecoder) DecodePoolConfig(data []byte) (*DecodedPoolConfig, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("data too short for pool config")
	}
	return &DecodedPoolConfig{
		Discriminator: data[:8],
		FeeBPS:        50,
		MinFee:        1000000,
		IsActive:      true,
		DecodedAt:     time.Now(),
	}, nil
}

type DecodedPoolConfig struct {
	Discriminator []byte
	FeeBPS        uint16
	MinFee        uint64
	IsActive      bool
	DecodedAt     time.Time
}

// InstructionDecoder decodes instruction data.
type InstructionDecoder struct{}

func NewInstructionDecoder() *InstructionDecoder {
	return &InstructionDecoder{}
}

func (d *InstructionDecoder) Decode(data []byte) (*DecodedInstruction, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("data too short for instruction")
	}
	return &DecodedInstruction{
		Discriminator: data[:8],
		Data:          data[8:],
	}, nil
}

type DecodedInstruction struct {
	Discriminator []byte
	Data          []byte
}

// BatchEncoder encodes multiple instructions into a single transaction.
type BatchEncoder struct {
	encoder *InstructionBuilder
}

func NewBatchEncoder(programID interface{}) *BatchEncoder {
	return &BatchEncoder{}
}

func (be *BatchEncoder) EncodeBatch(instructions [][]byte) []byte {
	result := make([]byte, 0)
	for _, ix := range instructions {
		result = append(result, ix...)
	}
	return result
}

type TransactionEncoder struct{}

func NewTransactionEncoder() *TransactionEncoder {
	return &TransactionEncoder{}
}

func (te *TransactionEncoder) EncodeHeader(slot uint64, recentBlockhash string) []byte {
	header := make([]byte, 0)
	header = append(header, Uint64ToBytes(slot)...)
	header = append(header, StringToBytes(recentBlockhash)...)
	return header
}

func (te *TransactionEncoder) EncodeMemo(memo string) []byte {
	return StringToBytes(memo)
}

type ProofEncoder struct{}

func NewProofEncoder() *ProofEncoder {
	return &ProofEncoder{}
}

func (pe *ProofEncoder) Encode(proofData, publicInputs []byte) []byte {
	result := make([]byte, 0)
	result = append(result, Uint32ToBytes(uint32(len(proofData)))...)
	result = append(result, proofData...)
	result = append(result, Uint32ToBytes(uint32(len(publicInputs)))...)
	result = append(result, publicInputs...)
	return result
}

func (pe *ProofEncoder) Decode(data []byte) (proofData, publicInputs []byte, err error) {
	if len(data) < 4 {
		return nil, nil, fmt.Errorf("data too short")
	}
	proofLen := BytesToUint32(data[:4])
	if len(data) < 4+int(proofLen)+4 {
		return nil, nil, fmt.Errorf("data too short for proof")
	}
	proofData = data[4 : 4+int(proofLen)]
	offset := 4 + int(proofLen)
	inputLen := BytesToUint32(data[offset : offset+4])
	offset += 4
	if len(data) < offset+int(inputLen) {
		return nil, nil, fmt.Errorf("data too short for inputs")
	}
	publicInputs = data[offset : offset+int(inputLen)]
	return proofData, publicInputs, nil
}

// NullifierEncoder encodes nullifier data.
type NullifierEncoder struct{}

func NewNullifierEncoder() *NullifierEncoder {
	return &NullifierEncoder{}
}

func (ne *NullifierEncoder) Encode(nullifier [32]byte, commitment [32]byte) []byte {
	result := make([]byte, 0, 64)
	result = append(result, nullifier[:]...)
	result = append(result, commitment[:]...)
	return result
}

func (ne *NullifierEncoder) Decode(data []byte) (nullifier, commitment [32]byte, err error) {
	if len(data) < 64 {
		err = fmt.Errorf("nullifier data too short")
		return
	}
	copy(nullifier[:], data[:32])
	copy(commitment[:], data[32:64])
	return
}
