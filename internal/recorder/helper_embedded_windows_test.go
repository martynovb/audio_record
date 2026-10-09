//go:build windows && embedded

package recorder

import (
	"bytes"
	"os"
	"testing"
)

func TestExtractEmbeddedWindowsHelper(t *testing.T) {
	if len(embeddedWindowsHelper) == 0 {
		t.Fatal("embedded Windows helper is empty")
	}

	path, err := extractEmbeddedWindowsHelperTo(t.TempDir())
	if err != nil {
		t.Fatalf("extractEmbeddedWindowsHelperTo returned an error: %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read extracted Windows helper: %v", err)
	}
	if !bytes.Equal(contents, embeddedWindowsHelper) {
		t.Fatal("extracted Windows helper does not match embedded helper")
	}
}
