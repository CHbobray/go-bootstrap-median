// Command gendata writes the three test data sets (positively skewed,
// symmetric, negatively skewed) used by both the R and the Go programs.
// Random numbers come from the Mersenne Twister.
//
//	gendata -n 100000 -dir data
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"github.com/seehuhn/mt19937"
)

// shape is one population to sample from.
type shape struct {
	name string
	draw func(rng *rand.Rand) float64
}

// shapes lists the three populations. The skewed pair are mirror images.
var shapes = []shape{
	{"positive_skew", func(rng *rand.Rand) float64 { return logNormal(rng) }},
	{"symmetric", func(rng *rand.Rand) float64 { return 50 + 10*rng.NormFloat64() }},
	{"negative_skew", func(rng *rand.Rand) float64 { return 150 - logNormal(rng) }},
}

// logNormal draws from a log-normal distribution with a long right tail.
func logNormal(rng *rand.Rand) float64 {
	return math.Exp(3.5 + 0.6*rng.NormFloat64())
}

// checkInput returns a plain-language description of the first problem with
// the command-line input, or "" when the input is usable.
func checkInput(n int, dir string) string {
	if n < 1 {
		return fmt.Sprintf("-n is %d, but each data set needs at least 1 observation (100 or more for a useful bootstrap).", n)
	}
	if dir == "" {
		return "-dir is empty. Give the folder to write the files to, for example -dir data"
	}
	return ""
}

// writeColumn writes a header and n draws to path, one per line. The 'g'
// format with precision -1 round-trips a float64 exactly, so R and Go read
// identical values.
func writeColumn(path string, n int, draw func() float64) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	fmt.Fprintln(writer, "x")
	for i := 0; i < n; i++ {
		writer.WriteString(strconv.FormatFloat(draw(), 'g', -1, 64))
		writer.WriteByte('\n')
	}
	return writer.Flush()
}

// generate writes one file per shape into dir and returns their paths.
func generate(n int, seed int64, dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	source := mt19937.New()
	source.Seed(seed)
	rng := rand.New(source)

	var paths []string
	for _, s := range shapes {
		path := filepath.Join(dir, fmt.Sprintf("%s_n%d.csv", s.name, n))
		if err := writeColumn(path, n, func() float64 { return s.draw(rng) }); err != nil {
			return paths, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func main() {
	n := flag.Int("n", 100_000, "observations per data set (1 or more)")
	seed := flag.Int64("seed", 20261109, "Mersenne Twister seed")
	dir := flag.String("dir", "data", "output folder")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "gendata: write the three bootstrap test data sets")
		fmt.Fprintln(os.Stderr, "\nUsage: gendata [-n 100000] [-seed 20261109] [-dir data]")
		flag.PrintDefaults()
	}
	flag.Parse()

	if problem := checkInput(*n, *dir); problem != "" {
		fmt.Fprintf(os.Stderr, "gendata: %s\n", problem)
		os.Exit(2)
	}
	paths, err := generate(*n, *seed, *dir)
	for _, path := range paths {
		fmt.Println("wrote", path)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "gendata: could not write the data: %v\n", err)
		os.Exit(1)
	}
}
