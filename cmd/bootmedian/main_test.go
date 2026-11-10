package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func sampleReport() report {
	var r report
	r.N, r.B, r.Workers = 3, 100, 2
	r.Median, r.SE, r.CILower, r.CIUpper = 2, 0.5, 1, 3
	r.File, r.Impl, r.Seed = "x.csv", "fast", 1
	return r
}

func TestWriteReportSummary(t *testing.T) {
	var out strings.Builder
	if err := writeReport(&out, sampleReport(), false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Median:          2.0000", "Standard error:  0.5000", "1.0000 to 3.0000", "x.csv"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary is missing %q:\n%s", want, out.String())
		}
	}
}

func TestWriteReportJSON(t *testing.T) {
	var out strings.Builder
	if err := writeReport(&out, sampleReport(), true); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if decoded["se"] != 0.5 || decoded["file"] != "x.csv" {
		t.Errorf("unexpected JSON: %v", decoded)
	}
}

// A full run on a small file, with a log file, both implementations.
func TestRunEndToEnd(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "x.csv")
	os.WriteFile(data, []byte("x\n5\n1\n4\n2\n3\n"), 0o644)
	logPath := filepath.Join(dir, "run.log")
	logger, closeLog, err := newLogger(logPath, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, impl := range []string{"naive", "fast"} {
		opts := options{data: data, b: 50, seed: 1, impl: impl, asJSON: true}
		if err := run(opts, logger); err != nil {
			t.Errorf("%s: %v", impl, err)
		}
	}
	closeLog()
	logged, _ := os.ReadFile(logPath)
	if got := strings.Count(string(logged), "bootstrap finished"); got != 2 {
		t.Errorf("log has %d 'bootstrap finished' lines, want 2", got)
	}

	empty := filepath.Join(dir, "empty.csv")
	os.WriteFile(empty, []byte("x\n"), 0o644)
	discard, _, _ := newLogger("", false)
	if err := run(options{data: empty, b: 50, impl: "fast"}, discard); err == nil {
		t.Error("run accepted a file with no numbers")
	}
}
