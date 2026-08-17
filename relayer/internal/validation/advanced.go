package validation

import (
	"fmt"
	"strings"
)

type ChainValidator struct {
	validators []ValidatorFunc
	errors     []string
}

type ValidatorFunc func() error

func NewChainValidator() *ChainValidator {
	return &ChainValidator{}
}

func (cv *ChainValidator) Add(fn ValidatorFunc) *ChainValidator {
	cv.validators = append(cv.validators, fn)
	return cv
}

func (cv *ChainValidator) ValidateAll() error {
	cv.errors = nil
	for _, v := range cv.validators {
		if err := v(); err != nil {
			cv.errors = append(cv.errors, err.Error())
		}
	}
	if len(cv.errors) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(cv.errors, "; "))
	}
	return nil
}

func (cv *ChainValidator) ValidateFirst() error {
	for _, v := range cv.validators {
		if err := v(); err != nil {
			return err
		}
	}
	return nil
}

func (cv *ChainValidator) ErrorCount() int {
	return len(cv.errors)
}

type ConditionalValidator struct {
	condition bool
	validator *Validator
}

func If(condition bool) *ConditionalValidator {
	return &ConditionalValidator{condition: condition, validator: New()}
}

func (cv *ConditionalValidator) Required(field, value string) *ConditionalValidator {
	if cv.condition {
		cv.validator.Required(field, value)
	}
	return cv
}

func (cv *ConditionalValidator) Range(field string, value, min, max int) *ConditionalValidator {
	if cv.condition {
		cv.validator.Range(field, value, min, max)
	}
	return cv
}

func (cv *ConditionalValidator) Valid() bool {
	return cv.validator.Valid()
}

func (cv *ConditionalValidator) Errors() []string {
	return cv.validator.Errors()
}

type CrossFieldValidator struct {
	errors []string
}

func NewCrossFieldValidator() *CrossFieldValidator {
	return &CrossFieldValidator{}
}

func (cfv *CrossFieldValidator) MustNotBeEqual(field1, val1, field2, val2 string) *CrossFieldValidator {
	if val1 == val2 {
		cfv.errors = append(cfv.errors, fmt.Sprintf("%s and %s must not be equal", field1, field2))
	}
	return cfv
}

func (cfv *CrossFieldValidator) MustBeEqual(field1, val1, field2, val2 string) *CrossFieldValidator {
	if val1 != val2 {
		cfv.errors = append(cfv.errors, fmt.Sprintf("%s and %s must be equal", field1, field2))
	}
	return cfv
}

func (cfv *CrossFieldValidator) OneRequired(fields ...string) *CrossFieldValidator {
	hasValue := false
	for _, f := range fields {
		if f != "" {
			hasValue = true
			break
		}
	}
	if !hasValue {
		cfv.errors = append(cfv.errors, fmt.Sprintf("at least one of the fields must be provided"))
	}
	return cfv
}

func (cfv *CrossFieldValidator) Valid() bool {
	return len(cfv.errors) == 0
}

func (cfv *CrossFieldValidator) Error() string {
	return strings.Join(cfv.errors, "; ")
}

type ValidationRule struct {
	Field   string
	Rule    string
	Value   interface{}
	Message string
}

type BulkValidator struct {
	rules  []ValidationRule
	errors []string
}

func NewBulkValidator() *BulkValidator {
	return &BulkValidator{}
}

func (bv *BulkValidator) AddRule(rule ValidationRule) *BulkValidator {
	bv.rules = append(bv.rules, rule)
	return bv
}

func (bv *BulkValidator) Validate() error {
	bv.errors = nil
	for _, rule := range bv.rules {
		switch rule.Rule {
		case "required":
			if rule.Value == nil || rule.Value == "" {
				bv.errors = append(bv.errors, rule.Message)
			}
		case "min":
			if v, ok := rule.Value.(int); ok {
				if v < rule.Value.(int) {
					bv.errors = append(bv.errors, rule.Message)
				}
			}
		}
	}
	if len(bv.errors) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(bv.errors, "; "))
	}
	return nil
}

func (bv *BulkValidator) ErrorCount() int {
	return len(bv.errors)
}

func (bv *BulkValidator) Errors() []string {
	return bv.errors
}
