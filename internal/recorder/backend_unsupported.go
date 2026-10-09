//go:build !darwin && !windows

package recorder

import (
	"context"
	"fmt"
	"io"
	"runtime"
)

type unsupportedBackend struct{}

func New() Backend {
	return &unsupportedBackend{}
}

func (backend *unsupportedBackend) Preflight() error {
	return fmt.Errorf("%w: %s", ErrNotImplemented, runtime.GOOS)
}

func (backend *unsupportedBackend) Capture(
	_ context.Context,
	_ Config,
	_ io.Writer,
	_ io.Writer,
) error {
	return backend.Preflight()
}
