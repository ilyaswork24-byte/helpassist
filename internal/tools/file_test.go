package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// autoConfirm returns a Confirmer that approves everything without reading stdin.
func autoConfirm() *Confirmer { return NewConfirmer(nil, true) }

func TestEditFileUniqueMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := editFile{confirm: autoConfirm()}
	out, err := e.Run(context.Background(), map[string]any{
		"path":       path,
		"old_string": "beta",
		"new_string": "BETA",
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(out, "изменён") {
		t.Errorf("unexpected result: %q", out)
	}
	data, _ := os.ReadFile(path)
	if got, want := string(data), "alpha\nBETA\ngamma\n"; got != want {
		t.Errorf("file = %q, want %q", got, want)
	}
}

func TestEditFileNoMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("alpha\n"), 0o644)

	e := editFile{confirm: autoConfirm()}
	out, err := e.Run(context.Background(), map[string]any{
		"path": path, "old_string": "missing", "new_string": "x",
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(out, "не найдена") {
		t.Errorf("expected not-found message, got %q", out)
	}
	if data, _ := os.ReadFile(path); string(data) != "alpha\n" {
		t.Error("file was modified despite no match")
	}
}

func TestEditFileAmbiguousMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	os.WriteFile(path, []byte("dup\ndup\n"), 0o644)

	e := editFile{confirm: autoConfirm()}
	out, err := e.Run(context.Background(), map[string]any{
		"path": path, "old_string": "dup", "new_string": "x",
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(out, "2 раз") {
		t.Errorf("expected ambiguity message, got %q", out)
	}
	if data, _ := os.ReadFile(path); string(data) != "dup\ndup\n" {
		t.Error("file was modified despite ambiguous match")
	}
}

func TestReadFileTruncationNote(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	big := strings.Repeat("a", maxReadBytes+100)
	os.WriteFile(path, []byte(big), 0o644)

	out, err := readFile{}.Run(context.Background(), map[string]any{"path": path})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !strings.Contains(out, "обрезан") {
		t.Error("expected truncation note for oversized file")
	}
}
