// Package anchor provides Anchor IDL type definitions for the Veilo Privacy Pool.
package anchor

import (
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

// Instruction represents an Anchor instruction.
type Instruction struct {
	Name          string
	Accounts      []AccountMeta
	InstructionData []byte
}

// AccountMeta represents account metadata for an instruction.
type AccountMeta struct {
	Pubkey     solana.PublicKey
	IsSigner   bool
	IsWritable bool
}

// InstructionAccounts are the common accounts for all instructions.
type InstructionAccounts struct {
	Pool          solana.PublicKey
	Vault         solana.PublicKey
	User          solana.PublicKey
	Authority     solana.PublicKey
	SystemProgram solana.PublicKey
	TokenProgram  solana.PublicKey
	Rent          solana.PublicKey
}

// BuildDepositInstruction builds a deposit instruction.
func BuildDepositInstruction(
	programID solana.PublicKey,
	pool solana.PublicKey,
	vault solana.PublicKey,
	user solana.PublicKey,
	amount uint64,
	proof []byte,
	publicInputs [][]byte,
) (*Instruction, error) {
	// Instruction discriminator for "deposit"
	discriminator := []byte{0xf2, 0x99, 0x07, 0x62, 0x13, 0x36, 0x05, 0x83}

	// Serialize instruction data
	data := make([]byte, 0, 8+8+len(proof)+len(publicInputs)*32)
	data = append(data, discriminator...)

	amountBytes := make([]byte, 8)
	amountBytes[0] = byte(amount)
	amountBytes[1] = byte(amount >> 8)
	amountBytes[2] = byte(amount >> 16)
	amountBytes[3] = byte(amount >> 24)
	amountBytes[4] = byte(amount >> 32)
	amountBytes[5] = byte(amount >> 40)
	amountBytes[6] = byte(amount >> 48)
	amountBytes[7] = byte(amount >> 56)
	data = append(data, amountBytes...)

	// Add proof
	proofLen := make([]byte, 4)
	proofLen[0] = byte(len(proof))
	data = append(data, proofLen...)
	data = append(data, proof...)

	// Add public inputs
	inputsLen := make([]byte, 4)
	inputsLen[0] = byte(len(publicInputs))
	data = append(data, inputsLen...)
	for _, input := range publicInputs {
		data = append(data, input[:32]...)
	}

	// Pool config PDA
	poolConfigPDA, _, _ := solana.FindProgramAddress(
		[][]byte{[]byte("pool_config"), pool.Bytes()},
		programID,
	)

	// Nullifier PDA (for the commitment)
	var nullifier [32]byte
	if len(publicInputs) > 2 {
		copy(nullifier[:], publicInputs[2][:32])
	}
	nullifierPDA, _, _ := solana.FindProgramAddress(
		[][]byte{[]byte("nullifier"), nullifier[:]},
		programID,
	)

	return &Instruction{
		Name: "deposit",
		Accounts: []AccountMeta{
			{Pubkey: poolConfigPDA, IsSigner: false, IsWritable: false},
			{Pubkey: pool, IsSigner: false, IsWritable: true},
			{Pubkey: vault, IsSigner: false, IsWritable: true},
			{Pubkey: nullifierPDA, IsSigner: false, IsWritable: true},
			{Pubkey: user, IsSigner: true, IsWritable: true},
			{Pubkey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
			{Pubkey: token.TokenProgramID, IsSigner: false, IsWritable: false},
		},
		InstructionData: data,
	}, nil
}

// BuildWithdrawInstruction builds a withdrawal instruction.
func BuildWithdrawInstruction(
	programID solana.PublicKey,
	pool solana.PublicKey,
	vault solana.PublicKey,
	recipient solana.PublicKey,
	relayer solana.PublicKey,
	amount uint64,
	fee uint64,
	proof []byte,
	publicInputs [][]byte,
) (*Instruction, error) {
	discriminator := []byte{0xb0, 0x9e, 0x05, 0x08, 0x44, 0x8f, 0xca, 0x35}

	data := make([]byte, 0, 8+8+8+8+32+32+len(proof)+len(publicInputs)*32)
	data = append(data, discriminator...)

	// amount
	amountBytes := make([]byte, 8)
	amountBytes[0] = byte(amount)
	amountBytes[1] = byte(amount >> 8)
	amountBytes[2] = byte(amount >> 16)
	amountBytes[3] = byte(amount >> 24)
	amountBytes[4] = byte(amount >> 32)
	amountBytes[5] = byte(amount >> 40)
	amountBytes[6] = byte(amount >> 48)
	amountBytes[7] = byte(amount >> 56)
	data = append(data, amountBytes...)

	// fee
	feeBytes := make([]byte, 8)
	feeBytes[0] = byte(fee)
	feeBytes[1] = byte(fee >> 8)
	feeBytes[2] = byte(fee >> 16)
	feeBytes[3] = byte(fee >> 24)
	feeBytes[4] = byte(fee >> 32)
	feeBytes[5] = byte(fee >> 40)
	feeBytes[6] = byte(fee >> 48)
	feeBytes[7] = byte(fee >> 56)
	data = append(data, feeBytes...)

	// recipient
	data = append(data, recipient.Bytes()...)

	// relayer
	data = append(data, relayer.Bytes()...)

	// proof
	proofLen := make([]byte, 4)
	proofLen[0] = byte(len(proof))
	data = append(data, proofLen...)
	data = append(data, proof...)

	// public inputs
	inputsLen := make([]byte, 4)
	inputsLen[0] = byte(len(publicInputs))
	data = append(data, inputsLen...)
	for _, input := range publicInputs {
		data = append(data, input[:32]...)
	}

	// PDAs
	poolConfigPDA, _, _ := solana.FindProgramAddress(
		[][]byte{[]byte("pool_config"), pool.Bytes()},
		programID,
	)

	var nullifier [32]byte
	if len(publicInputs) > 2 {
		copy(nullifier[:], publicInputs[2][:32])
	}
	nullifierPDA, _, _ := solana.FindProgramAddress(
		[][]byte{[]byte("nullifier"), nullifier[:]},
		programID,
	)

	relayerATA, _, _ := solana.FindAssociatedTokenAddress(relayer, pool)

	return &Instruction{
		Name: "withdraw",
		Accounts: []AccountMeta{
			{Pubkey: poolConfigPDA, IsSigner: false, IsWritable: false},
			{Pubkey: pool, IsSigner: false, IsWritable: true},
			{Pubkey: vault, IsSigner: false, IsWritable: true},
			{Pubkey: nullifierPDA, IsSigner: false, IsWritable: true},
			{Pubkey: recipient, IsSigner: false, IsWritable: true},
			{Pubkey: relayer, IsSigner: true, IsWritable: true},
			{Pubkey: relayerATA, IsSigner: false, IsWritable: true},
			{Pubkey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
			{Pubkey: token.TokenProgramID, IsSigner: false, IsWritable: false},
		},
		InstructionData: data,
	}, nil
}

// BuildSwapInstruction builds a Jupiter swap instruction.
func BuildSwapInstruction(
	programID solana.PublicKey,
	pool solana.PublicKey,
	vault solana.PublicKey,
	user solana.PublicKey,
	relayer solana.PublicKey,
	params SwapParamsData,
	proof []byte,
	publicInputs [][]byte,
	jupiterAccounts []solana.PublicKey,
) (*Instruction, error) {
	discriminator := []byte{0xf8, 0xc6, 0x9e, 0x91, 0xe1, 0x75, 0x87, 0xc8}

	data := make([]byte, 0, 128)
	data = append(data, discriminator...)

	// Serialize swap params
	amountInBytes := make([]byte, 8)
	amountInBytes[0] = byte(params.AmountIn)
	amountInBytes[1] = byte(params.AmountIn >> 8)
	amountInBytes[2] = byte(params.AmountIn >> 16)
	amountInBytes[3] = byte(params.AmountIn >> 24)
	amountInBytes[4] = byte(params.AmountIn >> 32)
	amountInBytes[5] = byte(params.AmountIn >> 40)
	amountInBytes[6] = byte(params.AmountIn >> 48)
	amountInBytes[7] = byte(params.AmountIn >> 56)
	data = append(data, amountInBytes...)

	// More serialization...
	data = append(data, proof...)

	poolConfigPDA, _, _ := solana.FindProgramAddress(
		[][]byte{[]byte("pool_config"), pool.Bytes()},
		programID,
	)

	accounts := []AccountMeta{
		{Pubkey: poolConfigPDA, IsSigner: false, IsWritable: false},
		{Pubkey: pool, IsSigner: false, IsWritable: true},
		{Pubkey: vault, IsSigner: false, IsWritable: true},
		{Pubkey: user, IsSigner: true, IsWritable: true},
		{Pubkey: relayer, IsSigner: true, IsWritable: true},
		{Pubkey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		{Pubkey: token.TokenProgramID, IsSigner: false, IsWritable: false},
	}

	// Add Jupiter accounts
	for _, acc := range jupiterAccounts {
		accounts = append(accounts, AccountMeta{
			Pubkey:     acc,
			IsSigner:   false,
			IsWritable: true,
		})
	}

	return &Instruction{
		Name:            "transact_swap",
		Accounts:        accounts,
		InstructionData: data,
	}, nil
}

// SwapParamsData represents swap parameters for instruction building.
type SwapParamsData struct {
	AmountIn     uint64
	MinAmountOut uint64
	Fee          uint64
}

// ConvertToSolanaInstructions converts our instructions to solana-go format.
func ConvertToSolanaInstructions(instrs []*Instruction) []solana.Instruction {
	result := make([]solana.Instruction, len(instrs))
	for i, instr := range instrs {
		accounts := make([]*solana.AccountMeta, len(instr.Accounts))
		for j, acc := range instr.Accounts {
			accounts[j] = &solana.AccountMeta{
				PublicKey:  acc.Pubkey,
				IsSigner:   acc.IsSigner,
				IsWritable: acc.IsWritable,
			}
		}
		result[i] = &solana.GenericInstruction{
			AccountValues: accounts,
			ProgramID:     solana.ProgramID{},
			DataBytes:     instr.InstructionData,
		}
	}
	return result
}
