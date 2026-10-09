package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"audio_record/internal/recorder"
)

type options struct {
	outputPath        string
	screenID          string
	microphoneID      string
	withoutSystem     bool
	withoutMicrophone bool
}

func Run(arguments []string, stdout, stderr io.Writer) int {
	options, err := parseOptions(arguments, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}

	if err := record(options, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "audio-record: %v\n", err)
		return 1
	}
	return 0
}

func parseOptions(arguments []string, stderr io.Writer) (options, error) {
	var result options
	flags := flag.NewFlagSet("audio-record", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&result.outputPath, "output", "", "output .m4a file (default: timestamped file in current directory)")
	flags.StringVar(&result.outputPath, "o", "", "alias for --output")
	flags.StringVar(&result.screenID, "screen-id", "", "macOS display ID (default: first display)")
	flags.StringVar(&result.microphoneID, "mic-id", "", "microphone device ID (default: built-in or first available)")
	flags.BoolVar(&result.withoutSystem, "no-system-audio", false, "do not capture system audio")
	flags.BoolVar(&result.withoutMicrophone, "no-microphone", false, "do not capture microphone audio")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: audio-record [options]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Records system audio and microphone audio into one M4A file.")
		fmt.Fprintln(stderr, "Press Ctrl+C to stop.")
		fmt.Fprintln(stderr)
		flags.PrintDefaults()
	}

	if err := flags.Parse(arguments); err != nil {
		return options{}, err
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n", flags.Arg(0))
		flags.Usage()
		return options{}, errors.New("unexpected positional argument")
	}
	if result.withoutSystem && result.withoutMicrophone {
		fmt.Fprintln(stderr, "at least one audio source must be enabled")
		return options{}, errors.New("no audio source enabled")
	}
	return result, nil
}

func record(options options, stdout, stderr io.Writer) error {
	outputPath, err := prepareOutputPath(options.outputPath)
	if err != nil {
		return err
	}
	backend := recorder.New()
	if err := backend.Preflight(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Fprintln(stdout, "Recording. Press Ctrl+C to stop.")
	err = backend.Capture(ctx, recorder.Config{
		OutputPath:   outputPath,
		ScreenID:     options.screenID,
		MicrophoneID: options.microphoneID,
		SystemAudio:  !options.withoutSystem,
		Microphone:   !options.withoutMicrophone,
	}, stdout, stderr)
	if err != nil {
		return err
	}

	if info, err := os.Stat(outputPath); err != nil || info.Size() == 0 {
		return errors.New("native capture did not produce an output file")
	}
	fmt.Fprintf(stdout, "Saved: %s\n", outputPath)
	return nil
}

func prepareOutputPath(configured string) (string, error) {
	if configured == "" {
		configured = fmt.Sprintf("recording-%s.m4a", time.Now().Format("20060102-150405"))
	}
	absolute, err := filepath.Abs(configured)
	if err != nil {
		return "", fmt.Errorf("resolve output path: %w", err)
	}
	if !strings.EqualFold(filepath.Ext(absolute), ".m4a") {
		return "", errors.New("output file must have the .m4a extension")
	}
	if _, err := os.Stat(absolute); err == nil {
		return "", fmt.Errorf("output already exists: %s", absolute)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect output path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}
	return absolute, nil
}
