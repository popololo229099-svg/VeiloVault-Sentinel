package validation

import (
	"fmt"
	"regexp"
	"strings"
)

type Validator struct {
	errors  []string
}

func New() *Validator {
	return &Validator{}
}

func (v *Validator) Required(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.errors = append(v.errors, fmt.Sprintf("%s is required", field))
	}
	return v
}

func (v *Validator) MinLength(field, value string, min int) *Validator {
	if len(value) < min {
		v.errors = append(v.errors, fmt.Sprintf("%s must be at least %d characters", field, min))
	}
	return v
}

func (v *Validator) MaxLength(field, value string, max int) *Validator {
	if len(value) > max {
		v.errors = append(v.errors, fmt.Sprintf("%s must be at most %d characters", field, max))
	}
	return v
}

func (v *Validator) Pattern(field, value, pattern string) *Validator {
	matched, _ := regexp.MatchString(pattern, value)
	if !matched {
		v.errors = append(v.errors, fmt.Sprintf("%s does not match pattern %s", field, pattern))
	}
	return v
}

func (v *Validator) Email(field, value string) *Validator {
	return v.Pattern(field, value, `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
}

func (v *Validator) Range(field string, value, min, max int) *Validator {
	if value < min || value > max {
		v.errors = append(v.errors, fmt.Sprintf("%s must be between %d and %d", field, min, max))
	}
	return v
}

func (v *Validator) Uint64Range(field string, value, min, max uint64) *Validator {
	if value < min || value > max {
		v.errors = append(v.errors, fmt.Sprintf("%s must be between %d and %d", field, min, max))
	}
	return v
}

func (v *Validator) Valid() bool {
	return len(v.errors) == 0
}

func (v *Validator) Errors() []string {
	return v.errors
}

func (v *Validator) Error() string {
	return strings.Join(v.errors, "; ")
}

type ValidateFunc func() error

func ValidateAll(fns ...ValidateFunc) error {
	var errs []string
	for _, fn := range fns {
		if err := fn(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(errs, "; "))
	}
	return nil
}
