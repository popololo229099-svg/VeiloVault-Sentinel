package validation

import (
	"fmt"
	"strings"

	"github.com/gagliardetto/solana-go"
)

type SolanaValidator struct{}

func NewSolanaValidator() *SolanaValidator {
	return &SolanaValidator{}
}

func (v *SolanaValidator) PublicKey(field, value string) *Validator {
	val := New()
	_, err := solana.PublicKeyFromBase58(value)
	if err != nil {
		val.errors = append(val.errors, fmt.Sprintf("%s is not a valid Solana public key", field))
	}
	return val
}

func (v *SolanaValidator) Amount(field string, amount, min, max uint64) *Validator {
	val := New()
	if amount < min {
		val.errors = append(val.errors, fmt.Sprintf("%s must be at least %d", field, min))
	}
	if max > 0 && amount > max {
		val.errors = append(val.errors, fmt.Sprintf("%s must be at most %d", field, max))
	}
	return val
}

func (v *SolanaValidator) Proof(field string, proof []byte) *Validator {
	val := New()
	if len(proof) == 0 {
		val.errors = append(val.errors, fmt.Sprintf("%s is required", field))
	}
	if len(proof) > 2048 {
		val.errors = append(val.errors, fmt.Sprintf("%s too large: %d bytes", field, len(proof)))
	}
	return val
}

func (v *SolanaValidator) TransactionType(field, value string) *Validator {
	val := New()
	validTypes := map[string]bool{
		"swap": true, "deposit": true, "withdraw": true,
		"position_open": true, "position_close": true,
		"perp_reissue": true, "perp_recover": true,
	}
	if !validTypes[value] {
		val.errors = append(val.errors, fmt.Sprintf("%s must be one of: swap, deposit, withdraw, position_open, position_close, perp_reissue, perp_recover", field))
	}
	return val
}

func (v *SolanaValidator) Signature(field, value string) *Validator {
	val := New()
	if len(value) < 64 || len(value) > 88 {
		val.errors = append(val.errors, fmt.Sprintf("%s must be a valid base58 signature", field))
		return val
	}
	for _, c := range value {
		if !strings.ContainsRune("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz", c) {
			val.errors = append(val.errors, fmt.Sprintf("%s contains invalid base58 character", field))
			return val
		}
	}
	return val
}

func (v *SolanaValidator) BPS(field string, bps uint16) *Validator {
	val := New()
	if bps > 10000 {
		val.errors = append(val.errors, fmt.Sprintf("%s must be between 0 and 10000", field))
	}
	return val
}

type RelayRequestValidator struct {
	solValidator *SolanaValidator
}

func NewRelayRequestValidator() *RelayRequestValidator {
	return &RelayRequestValidator{solValidator: NewSolanaValidator()}
}

func (v *RelayRequestValidator) ValidatePoolAddress(addr string) *Validator {
	return v.solValidator.PublicKey("pool", addr)
}

func (v *RelayRequestValidator) ValidateAmount(amount uint64) *Validator {
	return v.solValidator.Amount("amount", amount, 1, 0)
}

func (v *RelayRequestValidator) ValidateProof(proof []byte) *Validator {
	return v.solValidator.Proof("proof", proof)
}

func (v *RelayRequestValidator) ValidateType(txType string) *Validator {
	return v.solValidator.TransactionType("type", txType)
}

func (v *RelayRequestValidator) ValidateFee(fee uint64) *Validator {
	return v.solValidator.Amount("fee", fee, 0, 1000000000)
}
