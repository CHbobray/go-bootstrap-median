// Command bootstudy is the Go version of the course jump-start program
// run-bootstrap-median.R. It prints the same listing: standard errors of the
// mean and median for samples of four sizes from a normal population.
//
//	bootstudy -b 100
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/CHbobray/go-bootstrap-median/bootstrap"
)

// writeListing prints the study results in the layout of the R listing.
func writeListing(w io.Writer, cfg bootstrap.StudyConfig, rows []bootstrap.StudyRow) {
	fmt.Fprintf(w, "\nStudy conditions:\n  Population mean: %g SD: %g\n", cfg.PopMean, cfg.PopSD)
	fmt.Fprintf(w, "\nEstimated standard errors using %d bootstrap samples\n", cfg.B)
	for _, row := range rows {
		fmt.Fprintf(w, "\nSamples of size n = %d\n", row.N)
		fmt.Fprintf(w, "  SE Mean from Central Limit Theorem: %.2f\n", row.SEMeanCLT)
		fmt.Fprintf(w, "  SE Mean from Samples: %.2f\n", row.SEMeanSample)
		fmt.Fprintf(w, "  SE Mean from Bootstrap Samples: %.2f\n", row.SEMeanBoot)
		fmt.Fprintf(w, "  SE Median from Bootstrap Samples: %.2f\n", row.SEMedianBoot)
	}
}

func main() {
	b := flag.Int("b", 100, "bootstrap samples per sample (1 or more)")
	iterations := flag.Int("iterations", 100, "samples drawn per sample size (2 or more)")
	seed := flag.Int64("seed", 9999, "Mersenne Twister seed")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "bootstudy: Monte Carlo study of bootstrap standard errors")
		fmt.Fprintln(os.Stderr, "\nUsage: bootstudy [-b 100] [-iterations 100] [-seed 9999]")
		flag.PrintDefaults()
	}
	flag.Parse()

	cfg := bootstrap.StudyConfig{
		PopMean:     100,
		PopSD:       10,
		SampleSizes: []int{25, 100, 225, 400},
		Iterations:  *iterations,
		B:           *b,
		Seed:        *seed,
	}
	start := time.Now()
	rows, err := bootstrap.RunStudy(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstudy: -b must be at least 1 and -iterations at least 2 (got -b %d, -iterations %d)\n",
			*b, *iterations)
		os.Exit(2)
	}
	writeListing(os.Stdout, cfg, rows)
	fmt.Fprintf(os.Stderr, "\nstudy finished in %s\n", time.Since(start).Round(time.Microsecond))
}
