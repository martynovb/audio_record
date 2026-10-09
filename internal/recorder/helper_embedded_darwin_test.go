//go:build darwin && embedded

package recorder

import (
	"bytes"
	"os"
	"testing"
)

func TestExtractEmbeddedHelper(t *testing.T) {
	if len(embeddedMacOSHelper) == 0 {
		t.Fatal("embedded macOS helper is empty")
	}

	path, err := extractEmbeddedHelperTo(t.TempDir())
	if err != nil {
		t.Fatalf("extractEmbeddedHelperTo returned an error: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read extracted helper: %v", err)
	}
	if !bytes.Equal(contents, embeddedMacOSHelper) {
		t.Fatal("extracted helper does not match embedded helper")
	}
	if !isExecutable(path) {
		t.Fatal("extracted helper is not executable")
	}
}
