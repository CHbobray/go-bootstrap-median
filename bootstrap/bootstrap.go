// Package bootstrap estimates the standard error of the sample median with
// the nonparametric bootstrap (Efron 1979).
//
// Two implementations are provided:
//
//   - MedianSENaive is a line-for-line port of the R idiom
//     sd(replicate(B, median(sample(x, replace = TRUE)))). It allocates a new
//     slice and fully sorts it for every replicate, on one core.
//   - MedianSE is the tuned version that came out of benchmarking and
//     profiling: per-worker reusable buffers, quickselect in place of a
//     full sort, a multiply-shift bounded random integer, and goroutines.
//
// Both draw random numbers from the 64-bit Mersenne Twister
// (github.com/seehuhn/mt19937), the generator family used by R and Python.
package bootstrap

import (
	"errors"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/seehuhn/mt19937"
)

// Result summarises a bootstrap run.
type Result struct {
	N          int       `json:"n"`        // sample size
	B          int       `json:"b"`        // number of bootstrap replicates
	Median     float64   `json:"median"`   // median of the original sample
	SE         float64   `json:"se"`       // bootstrap standard error of the median
	Bias       float64   `json:"bias"`     // mean(replicates) - Median
	CILower    float64   `json:"ci_lower"` // 2.5th percentile of replicates
	CIUpper    float64   `json:"ci_upper"` // 97.5th percentile of replicates
	Workers    int       `json:"workers"`  // goroutines used
	Replicates []float64 `json:"-"`        // the B bootstrap medians
}

var (
	// ErrEmpty is returned when the data slice has no observations.
	ErrEmpty = errors.New("bootstrap: data is empty")
	// ErrReplicates is returned when fewer than two replicates are requested.
	ErrReplicates = errors.New("bootstrap: need at least 2 replicates")
	// ErrNaN is returned when the data contain NaN.
	ErrNaN = errors.New("bootstrap: data contain NaN")
)

// chunkSize is the number of replicates that share one seeded generator.
// Replicates are assigned to chunks by index, and chunk c is always seeded
// with the pair (seed, c), so the result is identical for any number of
// workers and runs with different seeds share no streams.
const chunkSize = 64

func validate(data []float64, b int) error {
	if len(data) == 0 {
		return ErrEmpty
	}
	if b < 2 {
		return ErrReplicates
	}
	for _, v := range data {
		if math.IsNaN(v) {
			return ErrNaN
		}
	}
	return nil
}

// Median returns the sample median without modifying data. For an even
// number of observations it averages the two middle values, as R does.
func Median(data []float64) float64 {
	buf := make([]float64, len(data))
	copy(buf, data)
	return medianInPlace(buf)
}

// medianSort is the textbook median: sort everything, read the middle.
func medianSort(buf []float64) float64 {
	sort.Float64s(buf)
	n := len(buf)
	if n%2 == 1 {
		return buf[n/2]
	}
	return (buf[n/2-1] + buf[n/2]) / 2
}

// medianInPlace finds the median in expected O(n) time with quickselect.
// It reorders buf.
func medianInPlace(buf []float64) float64 {
	n := len(buf)
	k := n / 2
	upper := selectK(buf, k)
	if n%2 == 1 {
		return upper
	}
	// After selectK every element left of k is <= buf[k], so the other
	// middle value is the largest of them.
	lower := buf[0]
	for _, v := range buf[1:k] {
		if v > lower {
			lower = v
		}
	}
	return (lower + upper) / 2
}

// selectK partially orders a so that a[k] holds the value it would hold if a
// were sorted, with nothing larger to its left and nothing smaller to its
// right. Hoare partitioning keeps it fast on the heavily tied data that
// resampling with replacement produces.
func selectK(a []float64, k int) float64 {
	lo, hi := 0, len(a)-1
	for lo < hi {
		// median-of-three pivot
		x, y, z := a[lo], a[lo+(hi-lo)/2], a[hi]
		p := max(min(x, y), min(max(x, y), z))
		i, j := lo, hi
		for i <= j {
			for a[i] < p {
				i++
			}
			for a[j] > p {
				j--
			}
			if i <= j {
				a[i], a[j] = a[j], a[i]
				i++
				j--
			}
		}
		switch {
		case k <= j:
			hi = j
		case k >= i:
			lo = i
		default:
			return a[k]
		}
	}
	return a[k]
}

// summarise turns the replicate medians into a Result. The percentile
// interval uses R's default quantile definition (type 7).
func summarise(data, reps []float64, workers int) Result {
	med := Median(data)
	var sum float64
	for _, v := range reps {
		sum += v
	}
	mean := sum / float64(len(reps))
	var ss float64
	for _, v := range reps {
		d := v - mean
		ss += d * d
	}
	sorted := make([]float64, len(reps))
	copy(sorted, reps)
	sort.Float64s(sorted)
	return Result{
		N:          len(data),
		B:          len(reps),
		Median:     med,
		SE:         math.Sqrt(ss / float64(len(reps)-1)),
		Bias:       mean - med,
		CILower:    quantile7(sorted, 0.025),
		CIUpper:    quantile7(sorted, 0.975),
		Workers:    workers,
		Replicates: reps,
	}
}

// quantile7 is R's quantile(x, p, type = 7) for sorted input.
func quantile7(sorted []float64, p float64) float64 {
	h := p * float64(len(sorted)-1)
	lo := int(math.Floor(h))
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	return sorted[lo] + (h-float64(lo))*(sorted[hi]-sorted[lo])
}

// MedianSENaive is the first, deliberately literal Go translation of the R
// code. It is kept as the baseline for the benchmarks.
func MedianSENaive(data []float64, b int, seed int64) (Result, error) {
	if err := validate(data, b); err != nil {
		return Result{}, err
	}
	src := mt19937.New()
	src.Seed(seed)
	rng := rand.New(src)
	n := len(data)
	reps := make([]float64, b)
	for r := 0; r < b; r++ {
		resample := make([]float64, n)
		for i := range resample {
			resample[i] = data[rng.Intn(n)]
		}
		reps[r] = medianSort(resample)
	}
	return summarise(data, reps, 1), nil
}

// fillChunks is the work of one goroutine. It claims chunks of replicates
// from nextChunk until none are left and writes each bootstrap median into
// reps. Workers write to disjoint parts of reps, so no lock is needed.
func fillChunks(data, reps []float64, seed int64, nextChunk *atomic.Int64) {
	b := len(reps)
	nChunks := (b + chunkSize - 1) / chunkSize
	buf := make([]float64, len(data)) // one buffer per worker, reused
	rng := mt19937.New()
	for {
		chunk := int(nextChunk.Add(1) - 1)
		if chunk >= nChunks {
			return
		}
		rng.SeedFromSlice([]uint64{uint64(seed), uint64(chunk)})
		end := min((chunk+1)*chunkSize, b)
		for r := chunk * chunkSize; r < end; r++ {
			resample(buf, data, rng)
			reps[r] = medianInPlace(buf)
		}
	}
}

// MedianSE is the tuned bootstrap. workers <= 0 means runtime.GOMAXPROCS(0).
// For a given seed the result does not depend on the number of workers.
func MedianSE(data []float64, b int, seed int64, workers int) (Result, error) {
	if err := validate(data, b); err != nil {
		return Result{}, err
	}
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	nChunks := (b + chunkSize - 1) / chunkSize
	if workers > nChunks {
		workers = nChunks
	}
	reps := make([]float64, b)

	var nextChunk atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fillChunks(data, reps, seed, &nextChunk)
		}()
	}
	wg.Wait()
	return summarise(data, reps, workers), nil
}
