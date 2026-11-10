package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckInput(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "x.csv")
	if err := os.WriteFile(good, []byte("x\n1\n2\n3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name          string
		data          string
		b, workers    int
		impl          string
		wantRejection bool
	}{
		{"valid", good, 100, 0, "fast", false},
		{"no file given", "", 100, 0, "fast", true},
		{"missing file", filepath.Join(dir, "nope.csv"), 100, 0, "fast", true},
		{"folder", dir, 100, 0, "fast", true},
		{"too few replicates", good, 1, 0, "fast", true},
		{"negative workers", good, 100, -1, "fast", true},
		{"unknown impl", good, 100, 0, "quick", true},
	}
	for _, c := range cases {
		problem := checkInput(c.data, c.b, c.workers, c.impl)
		if rejected := problem != ""; rejected != c.wantRejection {
			t.Errorf("%s: problem = %q, want rejection = %v", c.name, problem, c.wantRejection)
		}
	}
}

func TestReadColumn(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.csv")
	os.WriteFile(good, []byte("x\n1.5\n\n2\n-3e2\n"), 0o644)
	x, err := readColumn(good)
	if err != nil || len(x) != 3 || x[0] != 1.5 || x[2] != -300 {
		t.Errorf("readColumn(good) = %v, %v", x, err)
	}
	bad := filepath.Join(dir, "bad.csv")
	os.WriteFile(bad, []byte("x\n1\nabc\n"), 0o644)
	if _, err := readColumn(bad); err == nil {
		t.Error("readColumn accepted a non-numeric line")
	}
}
