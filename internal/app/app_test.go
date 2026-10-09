package app

import (
	"io"
	"testing"
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
