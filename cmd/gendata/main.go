// Command gendata writes the three test data sets (positively skewed,
// symmetric, negatively skewed) used by both the R and the Go programs.
// Random numbers come from the Mersenne Twister.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"github.com/seehuhn/mt19937"
)

func main() {
	n := flag.Int("n", 100_000, "observations per data set")
	seed := flag.Int64("seed", 20261109, "Mersenne Twister seed")
	dir := flag.String("dir", "data", "output directory")
	flag.Parse()

	src := mt19937.New()
	src.Seed(*seed)
	rng := rand.New(src)

	shapes := []struct {
		name string
		draw func() float64
	}{
		// log-normal: long right tail
		{"positive_skew", func() float64 { return math.Exp(3.5 + 0.6*rng.NormFloat64()) }},
		// normal
		{"symmetric", func() float64 { return 50 + 10*rng.NormFloat64() }},
		// mirrored log-normal: long left tail
		{"negative_skew", func() float64 { return 150 - math.Exp(3.5+0.6*rng.NormFloat64()) }},
	}

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		log.Fatal(err)
	}
	for _, s := range shapes {
		path := filepath.Join(*dir, fmt.Sprintf("%s_n%d.csv", s.name, *n))
		f, err := os.Create(path)
		if err != nil {
			log.Fatal(err)
		}
		w := bufio.NewWriter(f)
		fmt.Fprintln(w, "x")
		for i := 0; i < *n; i++ {
			// 'g' with precision -1 round-trips the float64 exactly, so R
			// and Go read identical values.
			w.WriteString(strconv.FormatFloat(s.draw(), 'g', -1, 64))
			w.WriteByte('\n')
		}
		if err := w.Flush(); err != nil {
			log.Fatal(err)
		}
		f.Close()
		log.Printf("wrote %s", path)
	}
}
