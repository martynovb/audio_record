package recorder

import (
	"context"
	"errors"
	"io"
)

var ErrNotImplemented = errors.New("audio recording is not implemented on this platform")

type Config struct {
	OutputPath   string
	ScreenID     string
	MicrophoneID string
	SystemAudio  bool
	Microphone   bool
}

// Backend owns native capture and encoding. The shared app package owns only
// the command-line contract and process lifecycle.
type Backend interface {
	Preflight() error
	Capture(ctx context.Context, config Config, stdout, stderr io.Writer) error
}
