package contextprovider

import "errors"

var (
	// ErrNoContext means no usable context snapshot is available (optional skip).
	ErrNoContext = errors.New("context: no snapshot available")

	// ErrInvalidContextTree means an explicit context directory failed validation.
	ErrInvalidContextTree = errors.New("context: invalid context tree")
)
