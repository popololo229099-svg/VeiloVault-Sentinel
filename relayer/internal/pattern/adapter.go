package pattern

import (
	"context"
	"fmt"
)

// Adapter pattern converts the interface of a class into another interface
// clients expect. It lets classes work together that otherwise couldn't
// because of incompatible interfaces.

// ExternalRelayer is an incompatible third-party relayer interface.
type ExternalRelayer interface {
	SendRawTransaction(data []byte) (string, error)
	QueryStatus(signature string) (int, error)
}

// InternalRelayer is our internal interface.
type InternalRelayer interface {
	SubmitTransaction(ctx context.Context, data []byte) (string, error)
	GetStatus(ctx context.Context, signature string) (string, error)
}

// ExternalRelayerAdapter adapts ExternalRelayer to InternalRelayer.
type ExternalRelayerAdapter struct {
	external ExternalRelayer
}

func NewExternalRelayerAdapter(external ExternalRelayer) *ExternalRelayerAdapter {
	return &ExternalRelayerAdapter{external: external}
}

func (a *ExternalRelayerAdapter) SubmitTransaction(ctx context.Context, data []byte) (string, error) {
	return a.external.SendRawTransaction(data)
}

func (a *ExternalRelayerAdapter) GetStatus(ctx context.Context, signature string) (string, error) {
	code, err := a.external.QueryStatus(signature)
	if err != nil {
		return "", err
	}
	switch code {
	case 0:
		return "pending", nil
	case 1:
		return "confirmed", nil
	case 2:
		return "finalized", nil
	default:
		return "failed", nil
	}
}

// ThirdPartyRelayer is a mock third-party implementation.
type ThirdPartyRelayer struct{}

func (r *ThirdPartyRelayer) SendRawTransaction(data []byte) (string, error) {
	return fmt.Sprintf("ext_sig_%x", data[:min(len(data), 4)]), nil
}

func (r *ThirdPartyRelayer) QueryStatus(signature string) (int, error) {
	return 1, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
