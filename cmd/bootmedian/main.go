// Command bootmedian reads a one-column CSV and prints the bootstrap
// standard error of the median as JSON.
//
//	bootmedian -data data/symmetric_n100000.csv -b 2000 -impl fast
//
// Structured logs go to stderr (and to -log if given). -cpuprofile and
// -memprofile write pprof files for `go tool pprof`.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/CHbobray/go-bootstrap-median/bootstrap"
)

type output struct {
	bootstrap.Result
	File         string  `json:"file"`
	Impl         string  `json:"impl"`
	Seed         int64   `json:"seed"`
	ReadSeconds  float64 `json:"read_seconds"`
	BootSeconds  float64 `json:"boot_seconds"`
	TotalAllocMB float64 `json:"total_alloc_mb"` // cumulative heap allocation
	HeapSysMB    float64 `json:"heap_sys_mb"`    // heap obtained from the OS
	NumGC        uint32  `json:"num_gc"`
}

func readColumn(path string) ([]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var x []float64
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		s := strings.TrimSpace(sc.Text())
		if s == "" {
			continue
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			if line == 1 { // header
				continue
			}
			return nil, fmt.Errorf("%s line %d: %w", path, line, err)
		}
		x = append(x, v)
	}
	return x, sc.Err()
}

// checkInput returns a plain-language description of the first problem with
// the command-line input, or "" when the input is usable.
func checkInput(data string, b, workers int, impl string) string {
	if data == "" {
		return "no data file given. Example: bootmedian -data data/symmetric_n100000.csv"
	}
	info, err := os.Stat(data)
	if err != nil {
		return fmt.Sprintf("cannot find data file %q. Check the path, or create the test data with gendata.", data)
	}
	if info.IsDir() {
		return fmt.Sprintf("%q is a folder. -data needs a one-column CSV file.", data)
	}
	if b < 2 {
		return fmt.Sprintf("-b is %d, but at least 2 bootstrap replicates are needed (1000 or more is typical).", b)
	}
	if workers < 0 {
		return fmt.Sprintf("-workers is %d. Use 0 for all cores or a positive number.", workers)
	}
	if impl != "naive" && impl != "fast" {
		return fmt.Sprintf("-impl is %q. Choose naive or fast.", impl)
	}
	return ""
}

func main() {
	data := flag.String("data", "", "one-column CSV file (required)")
	b := flag.Int("b", 2000, "bootstrap replicates")
	seed := flag.Int64("seed", 1, "Mersenne Twister seed")
	workers := flag.Int("workers", 0, "goroutines for -impl fast (0 = all cores)")
	impl := flag.String("impl", "fast", "naive or fast")
	logPath := flag.String("log", "", "also append JSON logs to this file")
	cpuProf := flag.String("cpuprofile", "", "write CPU profile here")
	memProf := flag.String("memprofile", "", "write allocation profile here")
	verbose := flag.Bool("v", false, "debug logging")
	flag.Parse()

	var logW io.Writer = os.Stderr
	if *logPath != "" {
		lf, err := os.OpenFile(*logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer lf.Close()
		logW = io.MultiWriter(os.Stderr, lf)
	}
	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(logW, &slog.HandlerOptions{Level: level}))
	fail := func(msg string, err error) {
		logger.Error(msg, "err", err)
		fmt.Fprintf(os.Stderr, "bootmedian: %s: %v\n", msg, err)
		os.Exit(1)
	}

	if problem := checkInput(*data, *b, *workers, *impl); problem != "" {
		fmt.Fprintf(os.Stderr, "bootmedian: %s\n\nRun 'bootmedian -h' for the full list of options.\n", problem)
		os.Exit(2)
	}
	if *memProf != "" {
		runtime.MemProfileRate = 4096 // sample finely enough to see small allocations
	}
	logger.Debug("start", "go", runtime.Version(), "gomaxprocs", runtime.GOMAXPROCS(0),
		"impl", *impl, "b", *b, "seed", *seed)

	t0 := time.Now()
	x, err := readColumn(*data)
	if err != nil {
		fail("could not read the data file (expected one number per line, with an optional header)", err)
	}
	if len(x) == 0 {
		fail("the data file has no numbers in it", bootstrap.ErrEmpty)
	}
	readSec := time.Since(t0).Seconds()
	logger.Info("data loaded", "file", *data, "n", len(x), "seconds", readSec)

	if *cpuProf != "" {
		f, err := os.Create(*cpuProf)
		if err != nil {
			fail("cpu profile", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fail("cpu profile", err)
		}
		defer pprof.StopCPUProfile()
	}

	t1 := time.Now()
	var res bootstrap.Result
	switch *impl {
	case "naive":
		res, err = bootstrap.MedianSENaive(x, *b, *seed)
	case "fast":
		res, err = bootstrap.MedianSE(x, *b, *seed, *workers)
	default:
		err = fmt.Errorf("unknown -impl %q", *impl)
	}
	if err != nil {
		fail("bootstrap failed", err)
	}
	bootSec := time.Since(t1).Seconds()

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	out := output{
		Result: res, File: *data, Impl: *impl, Seed: *seed,
		ReadSeconds: readSec, BootSeconds: bootSec,
		TotalAllocMB: float64(ms.TotalAlloc) / (1 << 20),
		HeapSysMB:    float64(ms.HeapSys) / (1 << 20),
		NumGC:        ms.NumGC,
	}
	logger.Info("bootstrap finished", "impl", *impl, "workers", res.Workers,
		"median", res.Median, "se", res.SE, "seconds", bootSec,
		"total_alloc_mb", out.TotalAllocMB, "num_gc", ms.NumGC)

	if *memProf != "" {
		f, err := os.Create(*memProf)
		if err != nil {
			fail("mem profile", err)
		}
		defer f.Close()
		if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
			fail("mem profile", err)
		}
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(out); err != nil {
		fail("encode", err)
	}
}
