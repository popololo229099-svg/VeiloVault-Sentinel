package encoder

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"context"
)

const (
	ProgramID = "GYy4kM6GHhpgLCUscuABbzkD2ZbJ2fneYryaZ6Ch7fFU"
)

var ProgramPubKey = solana.MustPublicKeyFromBase58(ProgramID)

type InstructionData struct {
	Discriminator [8]byte
	Data          []byte
}

func (d *InstructionData) Marshal() []byte {
	buf := new(bytes.Buffer)
	buf.Write(d.Discriminator[:])
	buf.Write(d.Data)
	return buf.Bytes()
}

func NewInstructionData(discriminator [8]byte, data []byte) *InstructionData {
	return &InstructionData{Discriminator: discriminator, Data: data}
}

func Uint64ToBytes(val uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, val)
	return b
}

func BytesToUint64(b []byte) uint64 {
	if len(b) < 8 {
		padded := make([]byte, 8)
		copy(padded, b)
		b = padded
	}
	return binary.LittleEndian.Uint64(b)
}

func Uint32ToBytes(val uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, val)
	return b
}

func BytesToUint32(b []byte) uint32 {
	if len(b) < 4 {
		padded := make([]byte, 4)
		copy(padded, b)
		b = padded
	}
	return binary.LittleEndian.Uint32(b)
}

func BoolToByte(val bool) byte {
	if val {
		return 1
	}
	return 0
}

func StringToBytes(s string) []byte {
	b := []byte(s)
	lenBytes := Uint32ToBytes(uint32(len(b)))
	return append(lenBytes, b...)
}

type PDADeriver struct {
	programID solana.PublicKey
}

func NewPDADeriver(programID solana.PublicKey) *PDADeriver {
	return &PDADeriver{programID: programID}
}

func (d *PDADeriver) Derive(seeds ...[]byte) (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(seeds, d.programID)
}

func (d *PDADeriver) DeriveWithBump(bump uint8, seeds ...[]byte) (solana.PublicKey, error) {
	allSeeds := make([][]byte, 0, len(seeds)+1)
	allSeeds = append(allSeeds, seeds...)
	allSeeds = append(allSeeds, []byte{bump})
	pubkey, err := solana.CreateProgramAddress(allSeeds, d.programID)
	return pubkey, err
}

type AccountMeta struct {
	Pubkey     solana.PublicKey
	IsSigner   bool
	IsWritable bool
}

func NewAccountMeta(pubkey solana.PublicKey, signer, writable bool) AccountMeta {
	return AccountMeta{Pubkey: pubkey, IsSigner: signer, IsWritable: writable}
}

func NewReadonlyAccountMeta(pubkey solana.PublicKey, signer bool) AccountMeta {
	return AccountMeta{Pubkey: pubkey, IsSigner: signer, IsWritable: false}
}

func NewWritableAccountMeta(pubkey solana.PublicKey) AccountMeta {
	return AccountMeta{Pubkey: pubkey, IsSigner: false, IsWritable: true}
}

type InstructionBuilder struct {
	programID solana.PublicKey
	data      []byte
	accounts  []AccountMeta
}

func NewInstructionBuilder(programID solana.PublicKey) *InstructionBuilder {
	return &InstructionBuilder{programID: programID}
}

func (b *InstructionBuilder) SetDiscriminator(disc [8]byte) *InstructionBuilder {
	b.data = append(b.data, disc[:]...)
	return b
}

func (b *InstructionBuilder) AppendUint64(val uint64) *InstructionBuilder {
	b.data = append(b.data, Uint64ToBytes(val)...)
	return b
}

func (b *InstructionBuilder) AppendUint32(val uint32) *InstructionBuilder {
	b.data = append(b.data, Uint32ToBytes(val)...)
	return b
}

func (b *InstructionBuilder) AppendBytes(data []byte) *InstructionBuilder {
	b.data = append(b.data, Uint32ToBytes(uint32(len(data)))...)
	b.data = append(b.data, data...)
	return b
}

func (b *InstructionBuilder) AppendPublicKey(pk solana.PublicKey) *InstructionBuilder {
	b.data = append(b.data, pk[:]...)
	return b
}

func (b *InstructionBuilder) AppendBool(val bool) *InstructionBuilder {
	b.data = append(b.data, BoolToByte(val))
	return b
}

func (b *InstructionBuilder) AppendRaw(data []byte) *InstructionBuilder {
	b.data = append(b.data, data...)
	return b
}

func (b *InstructionBuilder) AddAccount(meta AccountMeta) *InstructionBuilder {
	b.accounts = append(b.accounts, meta)
	return b
}

func (b *InstructionBuilder) Build() (solana.PublicKey, []AccountMeta, []byte) {
	return b.programID, b.accounts, b.data
}

type TransactionSigner struct {
	wallet *solana.Wallet
	client *rpc.Client
}

func NewTransactionSigner(wallet *solana.Wallet, rpcURL string) *TransactionSigner {
	return &TransactionSigner{
		wallet: wallet,
		client: rpc.New(rpcURL),
	}
}

func (ts *TransactionSigner) Sign(instructions []InstructionBuilder) (*solana.Transaction, error) {
	solanaInstructions := make([]solana.Instruction, 0, len(instructions))
	for _, ib := range instructions {
		progID, accounts, data := ib.Build()
		accountsMeta := make([]*solana.AccountMeta, len(accounts))
		for i, a := range accounts {
			accountsMeta[i] = &solana.AccountMeta{
				PublicKey:  a.Pubkey,
				IsSigner:   a.IsSigner,
				IsWritable: a.IsWritable,
			}
		}
		solanaInstructions = append(solanaInstructions, solana.NewInstruction(
			progID,
			solana.AccountMetaSlice(accountsMeta),
			data,
		))
	}

	recentBlockhash, err := ts.client.GetLatestBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return nil, fmt.Errorf("get blockhash: %w", err)
	}

	tx, err := solana.NewTransaction(
		solanaInstructions,
		recentBlockhash.Value.Blockhash,
		solana.TransactionPayer(ts.wallet.PublicKey()),
	)
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key == ts.wallet.PublicKey() {
			pk := ts.wallet.PrivateKey
			return &pk
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}

	return tx, nil
}

type NoteEncoder struct{}

func (e *NoteEncoder) Encode(amount uint64, salt [32]byte, recipient solana.PublicKey) []byte {
	buf := new(bytes.Buffer)
	buf.Write(Uint64ToBytes(amount))
	buf.Write(salt[:])
	buf.Write(recipient[:])
	return buf.Bytes()
}

func (e *NoteEncoder) Decode(data []byte) (amount uint64, salt [32]byte, recipient solana.PublicKey, err error) {
	if len(data) < 72 {
		err = fmt.Errorf("note data too short: %d bytes", len(data))
		return
	}
	amount = BytesToUint64(data[:8])
	copy(salt[:], data[8:40])
	copy(recipient[:], data[40:72])
	return
}

type FieldEncoder struct{}

func (e *FieldEncoder) ToBN254(val uint64) *big.Int {
	return new(big.Int).SetUint64(val)
}

func (e *FieldEncoder) FromBN254(val *big.Int) uint64 {
	return val.Uint64()
}

func (e *FieldEncoder) ValidateCanonical(val *big.Int) bool {
	q, _ := new(big.Int).SetString("21888242871839275222246405745257275088548364400416034343698204186575808495617", 10)
	return val.Sign() >= 0 && val.Cmp(q) < 0
}
