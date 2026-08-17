package pattern

import (
	"errors"
	"time"
)

var (
	ErrAlreadySubmitted         = errors.New("transaction already submitted")
	ErrAlreadyConfirmed         = errors.New("transaction already confirmed")
	ErrTransactionFailed        = errors.New("transaction is in failed state")
	ErrInvalidStateTransition   = errors.New("invalid state transition")
)

// Template Method pattern defines the skeleton of an algorithm in a base type,
// deferring specific steps to subclasses.

// TransactionPipeline is the template method.
type TransactionPipeline interface {
	Validate(input *PipelineInput) error
	BuildInstructions(input *PipelineInput) ([]*PipelineStep, error)
	Sign(tx *PipelineInput) error
	Submit(tx *PipelineInput) (string, error)
	PostProcess(tx *PipelineInput, signature string) error
}

// PipelineInput holds data flowing through the pipeline.
type PipelineInput struct {
	Type        string
	Pool        string
	Proof       []byte
	PublicInputs [][]byte
	Fee         uint64
	Amount      uint64
	Relayer     string
}

// PipelineStep represents one step in the pipeline.
type PipelineStep struct {
	Name     string
	Accounts []string
	Data     []byte
}

// PipelineRunner executes a pipeline with pre/post hooks.
type PipelineRunner struct {
	pipeline   TransactionPipeline
	beforeHook func(*PipelineInput) error
	afterHook  func(*PipelineInput, string) error
}

func NewPipelineRunner(pipeline TransactionPipeline) *PipelineRunner {
	return &PipelineRunner{pipeline: pipeline}
}

func (r *PipelineRunner) BeforeHook(fn func(*PipelineInput) error) {
	r.beforeHook = fn
}

func (r *PipelineRunner) AfterHook(fn func(*PipelineInput, string) error) {
	r.afterHook = fn
}

// Execute runs the full pipeline.
func (r *PipelineRunner) Execute(input *PipelineInput) (string, error) {
	start := time.Now()

	if r.beforeHook != nil {
		if err := r.beforeHook(input); err != nil {
			return "", err
		}
	}

	if err := r.pipeline.Validate(input); err != nil {
		return "", err
	}

	steps, err := r.pipeline.BuildInstructions(input)
	if err != nil {
		return "", err
	}
	_ = steps

	if err := r.pipeline.Sign(input); err != nil {
		return "", err
	}

	signature, err := r.pipeline.Submit(input)
	if err != nil {
		return "", err
	}

	if r.afterHook != nil {
		if err := r.afterHook(input, signature); err != nil {
			return signature, err
		}
	}

	_ = time.Since(start)
	return signature, nil
}

// DepositPipeline is a concrete implementation of TransactionPipeline.
type DepositPipeline struct{}

func (p *DepositPipeline) Validate(input *PipelineInput) error {
	if len(input.Proof) == 0 {
		return ErrInvalidInput
	}
	if input.Amount == 0 {
		return ErrInvalidInput
	}
	return nil
}

func (p *DepositPipeline) BuildInstructions(input *PipelineInput) ([]*PipelineStep, error) {
	return []*PipelineStep{{Name: "deposit", Data: input.Proof}}, nil
}

func (p *DepositPipeline) Sign(tx *PipelineInput) error { return nil }

func (p *DepositPipeline) Submit(tx *PipelineInput) (string, error) {
	return "simulated_sig", nil
}

func (p *DepositPipeline) PostProcess(tx *PipelineInput, signature string) error { return nil }

// WithdrawPipeline is another concrete implementation.
type WithdrawPipeline struct{}

func (p *WithdrawPipeline) Validate(input *PipelineInput) error {
	if len(input.Proof) == 0 {
		return ErrInvalidInput
	}
	return nil
}

func (p *WithdrawPipeline) BuildInstructions(input *PipelineInput) ([]*PipelineStep, error) {
	return []*PipelineStep{{Name: "withdraw"}}, nil
}

func (p *WithdrawPipeline) Sign(tx *PipelineInput) error { return nil }
func (p *WithdrawPipeline) Submit(tx *PipelineInput) (string, error) {
	return "simulated_sig", nil
}
func (p *WithdrawPipeline) PostProcess(tx *PipelineInput, signature string) error { return nil }

// SwapPipeline is another concrete implementation.
type SwapPipeline struct{}

func (p *SwapPipeline) Validate(input *PipelineInput) error {
	if len(input.Proof) == 0 {
		return ErrInvalidInput
	}
	return nil
}

func (p *SwapPipeline) BuildInstructions(input *PipelineInput) ([]*PipelineStep, error) {
	return []*PipelineStep{{Name: "swap"}}, nil
}

func (p *SwapPipeline) Sign(tx *PipelineInput) error { return nil }
func (p *SwapPipeline) Submit(tx *PipelineInput) (string, error) {
	return "simulated_sig", nil
}
func (p *SwapPipeline) PostProcess(tx *PipelineInput, signature string) error { return nil }
