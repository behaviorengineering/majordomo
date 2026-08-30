package contextgate

import (
	"context"
	"fmt"

	"github.com/behaviorengineering/strop/humanreview"
	"github.com/behaviorengineering/strop/regenerate"
)

// RegenOptions builds strop regenerate options from a gate reject reason.
func RegenOptions(reason string) (regenerate.RegenerateOptions, error) {
	return humanreview.RegenOptionsFromComment(context.Background(), humanreview.PassthroughNormalizer{}, reason)
}

// Gate wraps strop humanreview for optional future persistence.
type Gate struct {
	prefix string
}

// NewGate returns a gate helper with comment prefix.
func NewGate(prefix string) *Gate {
	if prefix == "" {
		prefix = DefaultPrefix
	}
	return &Gate{prefix: prefix}
}

// NormalizeReject returns the regen message for a reject reason.
func (g *Gate) NormalizeReject(reason string) (string, error) {
	opts, err := RegenOptions(reason)
	if err != nil {
		return "", err
	}
	if opts.Message == "" {
		return "", fmt.Errorf("empty reject reason")
	}
	return opts.Message, nil
}
