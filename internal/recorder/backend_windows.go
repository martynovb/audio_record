//go:build windows

package recorder

import (
	"context"
	"fmt"
	"io"
)

// windowsBackend is the extension point for the future WASAPI implementation.
// The public CLI and shared post-processing do not need to change when capture
// support is added here.
type windowsBackend struct{}

func New() Backend {
	return &windowsBackend{}
}

func (backend *windowsBackend) Preflight() error {
	return fmt.Errorf("%w: Windows backend is reserved for a future WASAPI implementation", ErrNotImplemented)
}

func (backend *windowsBackend) Capture(
	_ context.Context,
	_ Config,
	_ io.Writer,
	_ io.Writer,
) error {
	return backend.Preflight()
}
