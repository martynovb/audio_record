package app

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestParseOptions(t *testing.T) {
	result, err := parseOptions(
		[]string{"-o", "capture.m4a", "--screen-id", "42", "--no-microphone"},
		io.Discard,
	)
	if err != nil {
		t.Fatalf("parseOptions returned an error: %v", err)
	}
	if result.outputPath != "capture.m4a" {
		t.Fatalf("outputPath = %q, want capture.m4a", result.outputPath)
	}
	if result.screenID != "42" {
		t.Fatalf("screenID = %q, want 42", result.screenID)
	}
	if !result.withoutMicrophone {
		t.Fatal("withoutMicrophone = false, want true")
	}
}

func TestParseOptionsRequiresAnAudioSource(t *testing.T) {
	_, err := parseOptions(
		[]string{"--no-system-audio", "--no-microphone"},
		io.Discard,
	)
	if err == nil {
		t.Fatal("parseOptions returned nil, want an error")
	}
}

func TestMonitorRecordingDurationStopsWhenUserDeclines(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var output bytes.Buffer
	stopped := make(chan struct{})
	monitorRecordingDuration(
		ctx,
		strings.NewReader("no\n"),
		&output,
		time.Millisecond,
		time.Second,
		func() { close(stopped) },
	)

	select {
	case <-stopped:
	default:
		t.Fatal("recording was not stopped")
	}
	if !strings.Contains(output.String(), "Continue recording?") {
		t.Fatalf("output = %q, want continuation prompt", output.String())
	}
}

func TestMonitorRecordingDurationContinuesForAnotherInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var output bytes.Buffer
	stopCalls := 0
	monitorRecordingDuration(
		ctx,
		strings.NewReader("yes\nno\n"),
		&output,
		time.Millisecond,
		time.Second,
		func() { stopCalls++ },
	)

	if stopCalls != 1 {
		t.Fatalf("stop calls = %d, want 1", stopCalls)
	}
	if prompts := strings.Count(output.String(), "Continue recording?"); prompts != 2 {
		t.Fatalf("prompt count = %d, want 2; output = %q", prompts, output.String())
	}
	if !strings.Contains(output.String(), "Continuing recording.") {
		t.Fatalf("output = %q, want continuation confirmation", output.String())
	}
}

func TestMonitorRecordingDurationStopsAfterResponseTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()

	var output bytes.Buffer
	stopped := make(chan struct{})
	monitorRecordingDuration(
		ctx,
		reader,
		&output,
		time.Millisecond,
		5*time.Millisecond,
		func() { close(stopped) },
	)

	select {
	case <-stopped:
	default:
		t.Fatal("recording was not stopped")
	}
	if !strings.Contains(output.String(), "No response received within 5 minutes") {
		t.Fatalf("output = %q, want timeout message", output.String())
	}
}
