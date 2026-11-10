package main

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckInput(t *testing.T) {
	if checkInput(100, "data") != "" {
		t.Error("valid input was rejected")
	}
	if checkInput(0, "data") == "" || checkInput(-5, "data") == "" {
		t.Error("n below 1 was accepted")
	}
	if checkInput(100, "") == "" {
		t.Error("empty folder was accepted")
	}
}

func TestGenerateWritesThreeFiles(t *testing.T) {
	const n = 200
	paths, err := generate(n, 1, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("wrote %d files, want 3", len(paths))
	}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if lines := bytes.Count(content, []byte("\n")); lines != n+1 {
			t.Errorf("%s has %d lines, want header + %d", filepath.Base(path), lines, n)
		}
	}
}

// The same seed must give byte-identical files, so R and Go always see the
// same data.
func TestGenerateIsReproducible(t *testing.T) {
	hash := func() [32]byte {
		paths, err := generate(100, 7, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		var all []byte
		for _, path := range paths {
			content, _ := os.ReadFile(path)
			all = append(all, content...)
		}
		return sha256.Sum256(all)
	}
	if hash() != hash() {
		t.Error("two runs with the same seed wrote different data")
	}
}
