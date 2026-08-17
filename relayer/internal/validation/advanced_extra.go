package validation

import (
	"fmt"
	"regexp"
	"strings"
)

type RegexValidator struct {
	patterns map[string]*regexp.Regexp
}

func NewRegexValidator() *RegexValidator {
	return &RegexValidator{patterns: make(map[string]*regexp.Regexp)}
}

func (rv *RegexValidator) Register(name, pattern string) error {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern %s: %w", pattern, err)
	}
	rv.patterns[name] = compiled
	return nil
}

func (rv *RegexValidator) Validate(name, value string) bool {
	pattern, ok := rv.patterns[name]
	if !ok {
		return false
	}
	return pattern.MatchString(value)
}

type CompoundValidator struct {
	validators []func(string) error
}

func NewCompoundValidator() *CompoundValidator {
	return &CompoundValidator{}
}

func (cv *CompoundValidator) Add(validator func(string) error) *CompoundValidator {
	cv.validators = append(cv.validators, validator)
	return cv
}

func (cv *CompoundValidator) Validate(value string) error {
	for _, v := range cv.validators {
		if err := v(value); err != nil {
			return err
		}
	}
	return nil
}

func RequiredField(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

func MinLength(field, value string, min int) error {
	if len(value) < min {
		return fmt.Errorf("%s must be at least %d characters", field, min)
	}
	return nil
}

func MaxLength(field, value string, max int) error {
	if len(value) > max {
		return fmt.Errorf("%s must be at most %d characters", field, max)
	}
	return nil
}

func EmailFormat(field, value string) error {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(value) {
		return fmt.Errorf("%s must be a valid email address", field)
	}
	return nil
}

func URLFormat(field, value string) error {
	urlRegex := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	if !urlRegex.MatchString(value) {
		return fmt.Errorf("%s must be a valid URL", field)
	}
	return nil
}

type FieldError struct {
	Field   string
	Message string
}

func (fe FieldError) Error() string {
	return fmt.Sprintf("%s: %s", fe.Field, fe.Message)
}

type ValidationError struct {
	Errors []FieldError
}

func NewValidationError() *ValidationError {
	return &ValidationError{Errors: make([]FieldError, 0)}
}

func (ve *ValidationError) Add(field, message string) {
	ve.Errors = append(ve.Errors, FieldError{Field: field, Message: message})
}

func (ve *ValidationError) HasErrors() bool {
	return len(ve.Errors) > 0
}

func (ve *ValidationError) Error() string {
	messages := make([]string, len(ve.Errors))
	for i, e := range ve.Errors {
		messages[i] = e.Error()
	}
	return strings.Join(messages, "; ")
}

func (ve *ValidationError) ErrorsByField() map[string]string {
	result := make(map[string]string)
	for _, e := range ve.Errors {
		result[e.Field] = e.Message
	}
	return result
}

func (ve *ValidationError) FieldErrors(field string) []string {
	var result []string
	for _, e := range ve.Errors {
		if e.Field == field {
			result = append(result, e.Message)
		}
	}
	return result
}
