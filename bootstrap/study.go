package bootstrap

import (
	"errors"
	"math"
	"math/bits"
	"math/rand"

	"github.com/seehuhn/mt19937"
)

// StudyConfig describes the Monte Carlo study from the course jump-start
// program run-bootstrap-median.R: repeated samples from a normal population,
// each one bootstrapped for its mean and median.
type StudyConfig struct {
	PopMean     float64
	PopSD       float64
	SampleSizes []int
	Iterations  int   // independent samples drawn per sample size
	B           int   // bootstrap resamples per sample
	Seed        int64 // Mersenne Twister seed
}

// StudyRow holds the standard error estimates for one sample size.
type StudyRow struct {
	N            int
	SEMeanCLT    float64 // PopSD / sqrt(n)
	SEMeanSample float64 // SD of the sample means across iterations
	SEMeanBoot   float64 // SD of the mean bootstrap mean across iterations
	SEMedianBoot float64 // SD of the mean bootstrap median across iterations
}

// ErrStudyConfig is returned when a StudyConfig cannot be run.
var ErrStudyConfig = errors.New("bootstrap: study needs PopSD > 0, Iterations >= 2, B >= 1 and positive sample sizes")

func (c StudyConfig) valid() bool {
	if c.PopSD <= 0 || c.Iterations < 2 || c.B < 1 || len(c.SampleSizes) == 0 {
		return false
	}
	for _, n := range c.SampleSizes {
		if n < 1 {
			return false
		}
	}
	return true
}

// Mean returns the arithmetic mean of data.
func Mean(data []float64) float64 {
	var sum float64
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// StdDev returns the sample standard deviation (n-1 denominator), as R's sd.
func StdDev(data []float64) float64 {
	mean := Mean(data)
	var ss float64
	for _, v := range data {
		d := v - mean
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(data)-1))
}

// resample fills dst with len(dst) draws from data, with replacement.
func resample(dst, data []float64, rng *mt19937.MT19937) {
	n := uint64(len(data))
	for i := range dst {
		idx, _ := bits.Mul64(rng.Uint64(), n)
		dst[i] = data[idx]
	}
}

// RunStudy repeats the jump-start study and returns one row per sample size,
// in the order given.
func RunStudy(cfg StudyConfig) ([]StudyRow, error) {
	if !cfg.valid() {
		return nil, ErrStudyConfig
	}
	src := mt19937.New()
	src.Seed(cfg.Seed)
	normal := rand.New(src)

	nSizes := len(cfg.SampleSizes)
	sampleMeans := make([][]float64, nSizes)
	bootMeans := make([][]float64, nSizes)
	bootMedians := make([][]float64, nSizes)

	for iter := 0; iter < cfg.Iterations; iter++ {
		for s, n := range cfg.SampleSizes {
			sample := make([]float64, n)
			for i := range sample {
				sample[i] = cfg.PopMean + cfg.PopSD*normal.NormFloat64()
			}
			buf := make([]float64, n)
			var sumMean, sumMedian float64
			for b := 0; b < cfg.B; b++ {
				resample(buf, sample, src)
				sumMean += Mean(buf)
				sumMedian += medianInPlace(buf)
			}
			sampleMeans[s] = append(sampleMeans[s], Mean(sample))
			bootMeans[s] = append(bootMeans[s], sumMean/float64(cfg.B))
			bootMedians[s] = append(bootMedians[s], sumMedian/float64(cfg.B))
		}
	}

	rows := make([]StudyRow, nSizes)
	for s, n := range cfg.SampleSizes {
		rows[s] = StudyRow{
			N:            n,
			SEMeanCLT:    cfg.PopSD / math.Sqrt(float64(n)),
			SEMeanSample: StdDev(sampleMeans[s]),
			SEMeanBoot:   StdDev(bootMeans[s]),
			SEMedianBoot: StdDev(bootMedians[s]),
		}
	}
	return rows, nil
}
