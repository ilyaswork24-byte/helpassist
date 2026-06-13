package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsBinary(t *testing.T) {
	if isBinary([]byte("plain text\nmore text")) {
		t.Error("text wrongly detected as binary")
	}
	if !isBinary([]byte{'a', 'b', 0, 'c'}) {
		t.Error("NUL-containing data not detected as binary")
	}
	if isBinary(nil) {
		t.Error("empty data should not be binary")
	}
}

func TestGrepFindsMatches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n// TODO: fix\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("nothing here\n"), 0o644)

	out, err := grepTool{}.Run(context.Background(), map[string]any{
		"pattern": "TODO", "dir": dir,
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "TODO") {
		t.Errorf("expected match in a.go, got %q", out)
	}
}

func TestGrepGlobFilter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("needle\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("needle\n"), 0o644)

	out, err := grepTool{}.Run(context.Background(), map[string]any{
		"pattern": "needle", "dir": dir, "glob": "*.go",
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if strings.Contains(out, "b.txt") {
		t.Errorf("glob filter leaked non-matching file: %q", out)
	}
	if !strings.Contains(out, "a.go") {
		t.Errorf("glob filter dropped matching file: %q", out)
	}
}
