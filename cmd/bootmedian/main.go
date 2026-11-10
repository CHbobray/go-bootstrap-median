// Command bootmedian reads a one-column CSV file and reports the bootstrap
// standard error of the median.
//
//	bootmedian -data data/symmetric_n100000.csv
//	bootmedian -data data/symmetric_n100000.csv -b 5000 -json
//
// By default it prints a short readable summary. -json prints one line of
// JSON instead. -v shows progress logs on the screen and -log appends them
// to a file. -cpuprofile and -memprofile write pprof files for
// `go tool pprof`.
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

// options holds the command-line settings.
type options struct {
	data    string
	b       int
	seed    int64
	workers int
	impl    string
	logPath string
	cpuProf string
	memProf string
	verbose bool
	asJSON  bool
}

// report is everything the program prints about one run.
type report struct {
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

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.data, "data", "", "one-column CSV file (required)")
	flag.IntVar(&opts.b, "b", 2000, "bootstrap replicates (2 or more)")
	flag.Int64Var(&opts.seed, "seed", 1, "Mersenne Twister seed")
	flag.IntVar(&opts.workers, "workers", 0, "goroutines for -impl fast (0 = all cores)")
	flag.StringVar(&opts.impl, "impl", "fast", "naive or fast")
	flag.BoolVar(&opts.asJSON, "json", false, "print the result as one line of JSON")
	flag.BoolVar(&opts.verbose, "v", false, "show progress logs on the screen")
	flag.StringVar(&opts.logPath, "log", "", "append JSON logs to this file")
	flag.StringVar(&opts.cpuProf, "cpuprofile", "", "write a CPU profile to this file")
	flag.StringVar(&opts.memProf, "memprofile", "", "write an allocation profile to this file")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "bootmedian: bootstrap standard error of the median")
		fmt.Fprintln(os.Stderr, "\nUsage: bootmedian -data FILE [options]")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		flag.PrintDefaults()
	}
	flag.Parse()
	return opts
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

// newLogger sends JSON logs to the screen when verbose is set and to the
// file at logPath when one is given. With neither, logs are discarded. The
// returned function closes the log file.
func newLogger(logPath string, verbose bool) (*slog.Logger, func(), error) {
	var writers []io.Writer
	closeLog := func() {}
	if verbose {
		writers = append(writers, os.Stderr)
	}
	if logPath != "" {
		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, closeLog, err
		}
		writers = append(writers, file)
		closeLog = func() { file.Close() }
	}
	var dest io.Writer = io.Discard
	if len(writers) > 0 {
		dest = io.MultiWriter(writers...)
	}
	return slog.New(slog.NewJSONHandler(dest, nil)), closeLog, nil
}

// readColumn reads one number per line. A first line that is not a number
// is treated as a header.
func readColumn(path string) ([]float64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var values []float64
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			if line == 1 {
				continue
			}
			return nil, fmt.Errorf("line %d is not a number: %q", line, text)
		}
		values = append(values, value)
	}
	return values, scanner.Err()
}

// startCPUProfile begins profiling to path and returns the function that
// stops it. An empty path does nothing.
func startCPUProfile(path string) (stop func(), err error) {
	if path == "" {
		return func() {}, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		file.Close()
		return nil, err
	}
	return func() {
		pprof.StopCPUProfile()
		file.Close()
	}, nil
}

// writeMemProfile saves the allocation profile to path. An empty path does
// nothing.
func writeMemProfile(path string) error {
	if path == "" {
		return nil
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return pprof.Lookup("allocs").WriteTo(file, 0)
}

// runBootstrap times the chosen implementation on values.
func runBootstrap(values []float64, opts options) (bootstrap.Result, time.Duration, error) {
	start := time.Now()
	var result bootstrap.Result
	var err error
	if opts.impl == "naive" {
		result, err = bootstrap.MedianSENaive(values, opts.b, opts.seed)
	} else {
		result, err = bootstrap.MedianSE(values, opts.b, opts.seed, opts.workers)
	}
	return result, time.Since(start), err
}

// buildReport adds timings and Go runtime memory statistics to the result.
func buildReport(result bootstrap.Result, opts options, readTime, bootTime time.Duration) report {
	const bytesPerMB = 1 << 20
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return report{
		Result:       result,
		File:         opts.data,
		Impl:         opts.impl,
		Seed:         opts.seed,
		ReadSeconds:  readTime.Seconds(),
		BootSeconds:  bootTime.Seconds(),
		TotalAllocMB: float64(mem.TotalAlloc) / bytesPerMB,
		HeapSysMB:    float64(mem.HeapSys) / bytesPerMB,
		NumGC:        mem.NumGC,
	}
}

// writeSummary prints the report for a person to read.
func writeSummary(w io.Writer, r report) {
	fmt.Fprintln(w, "Bootstrap standard error of the median")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Data file:       %s\n", r.File)
	fmt.Fprintf(w, "  Observations:    %d\n", r.N)
	fmt.Fprintf(w, "  Replicates:      %d (seed %d)\n", r.B, r.Seed)
	fmt.Fprintf(w, "  Method:          %s, %d worker(s)\n", r.Impl, r.Workers)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Median:          %.4f\n", r.Median)
	fmt.Fprintf(w, "  Standard error:  %.4f\n", r.SE)
	fmt.Fprintf(w, "  Bias:            %.4f\n", r.Bias)
	fmt.Fprintf(w, "  95%% interval:    %.4f to %.4f\n", r.CILower, r.CIUpper)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Time:            %.2f s bootstrap, %.2f s reading data\n", r.BootSeconds, r.ReadSeconds)
	fmt.Fprintf(w, "  Memory:          %.1f MB allocated, %d garbage collections\n", r.TotalAllocMB, r.NumGC)
}

// writeReport prints the report as JSON or as a readable summary.
func writeReport(w io.Writer, r report, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(w).Encode(r)
	}
	writeSummary(w, r)
	return nil
}

// run does the work of the program after the input has been checked.
func run(opts options, logger *slog.Logger) error {
	if opts.memProf != "" {
		runtime.MemProfileRate = 4096 // sample finely enough to see small allocations
	}

	readStart := time.Now()
	values, err := readColumn(opts.data)
	if err != nil {
		return fmt.Errorf("could not read %s (expected one number per line, with an optional header): %w", opts.data, err)
	}
	if len(values) == 0 {
		return fmt.Errorf("%s has no numbers in it", opts.data)
	}
	readTime := time.Since(readStart)
	logger.Info("data loaded", "file", opts.data, "n", len(values), "seconds", readTime.Seconds())

	stopCPUProfile, err := startCPUProfile(opts.cpuProf)
	if err != nil {
		return fmt.Errorf("could not start the CPU profile: %w", err)
	}
	result, bootTime, err := runBootstrap(values, opts)
	stopCPUProfile()
	if err != nil {
		return err
	}

	summary := buildReport(result, opts, readTime, bootTime)
	logger.Info("bootstrap finished", "impl", opts.impl, "workers", result.Workers,
		"median", result.Median, "se", result.SE, "seconds", summary.BootSeconds,
		"total_alloc_mb", summary.TotalAllocMB, "num_gc", summary.NumGC)

	if err := writeMemProfile(opts.memProf); err != nil {
		return fmt.Errorf("could not write the memory profile: %w", err)
	}
	return writeReport(os.Stdout, summary, opts.asJSON)
}

func main() {
	opts := parseFlags()
	if problem := checkInput(opts.data, opts.b, opts.workers, opts.impl); problem != "" {
		fmt.Fprintf(os.Stderr, "bootmedian: %s\n\nRun 'bootmedian -h' for the full list of options.\n", problem)
		os.Exit(2)
	}
	logger, closeLog, err := newLogger(opts.logPath, opts.verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootmedian: could not open the log file: %v\n", err)
		os.Exit(1)
	}
	defer closeLog()

	if err := run(opts, logger); err != nil {
		logger.Error("run failed", "err", err)
		fmt.Fprintf(os.Stderr, "bootmedian: %v\n", err)
		closeLog()
		os.Exit(1)
	}
}
