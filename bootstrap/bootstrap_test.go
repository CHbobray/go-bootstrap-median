package bootstrap

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"testing"

	"github.com/seehuhn/mt19937"
)

func normalData(n int, seed int64) []float64 {
	src := mt19937.New()
	src.Seed(seed)
	rng := rand.New(src)
	x := make([]float64, n)
	for i := range x {
		x[i] = 50 + 10*rng.NormFloat64()
	}
	return x
}

func TestMedianKnownValues(t *testing.T) {
	cases := []struct {
		in   []float64
		want float64
	}{
		{[]float64{7}, 7},
		{[]float64{2, 1}, 1.5},
		{[]float64{3, 1, 2}, 2},
		{[]float64{4, 1, 3, 2}, 2.5},
		{[]float64{5, 5, 5, 5, 1}, 5},
		{[]float64{-1, -3, 10, 0}, -0.5},
	}
	for _, c := range cases {
		orig := append([]float64(nil), c.in...)
		if got := Median(c.in); got != c.want {
			t.Errorf("Median(%v) = %v, want %v", orig, got, c.want)
		}
		for i := range orig {
			if c.in[i] != orig[i] {
				t.Errorf("Median modified its input: %v -> %v", orig, c.in)
				break
			}
		}
	}
}

// Quickselect must agree with a full sort, including on tied data like
// the resamples the bootstrap produces.
func TestQuickselectMatchesSort(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := 0; trial < 2000; trial++ {
		n := 1 + rng.Intn(200)
		a := make([]float64, n)
		for i := range a {
			a[i] = float64(rng.Intn(1 + n/3)) // many ties
		}
		b := append([]float64(nil), a...)
		want := medianSort(b)
		if got := medianInPlace(a); got != want {
			t.Fatalf("trial %d n=%d: quickselect %v, sort %v", trial, n, got, want)
		}
	}
}

func TestQuantile7MatchesR(t *testing.T) {
	// R: quantile(1:10, c(.025, .5, .975)) -> 1.225, 5.5, 9.775
	x := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	for p, want := range map[float64]float64{0.025: 1.225, 0.5: 5.5, 0.975: 9.775, 1: 10} {
		if got := quantile7(x, p); math.Abs(got-want) > 1e-12 {
			t.Errorf("quantile7(p=%v) = %v, want %v", p, got, want)
		}
	}
}

func TestInputValidation(t *testing.T) {
	if _, err := MedianSE(nil, 100, 1, 1); !errors.Is(err, ErrEmpty) {
		t.Errorf("empty data: got %v", err)
	}
	if _, err := MedianSE([]float64{1, 2}, 1, 1, 1); !errors.Is(err, ErrReplicates) {
		t.Errorf("B=1: got %v", err)
	}
	if _, err := MedianSENaive([]float64{1, math.NaN()}, 10, 1); !errors.Is(err, ErrNaN) {
		t.Errorf("NaN: got %v", err)
	}
}

// Same seed, any number of workers: bit-identical replicates.
func TestDeterministicAcrossWorkers(t *testing.T) {
	x := normalData(501, 3)
	base, err := MedianSE(x, 1000, 42, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []int{2, 3, 8} {
		got, _ := MedianSE(x, 1000, 42, w)
		for i := range base.Replicates {
			if got.Replicates[i] != base.Replicates[i] {
				t.Fatalf("workers=%d: replicate %d differs", w, i)
			}
		}
	}
	other, _ := MedianSE(x, 1000, 43, 1)
	if other.SE == base.SE {
		t.Error("different seeds gave the same SE")
	}
}

// For normal data the SE of the median is about 1.2533*sigma/sqrt(n).
func TestSEMatchesAsymptoticTheory(t *testing.T) {
	const n, sigma = 4001, 10.0
	x := normalData(n, 7)
	want := 1.2533 * sigma / math.Sqrt(n)
	fast, _ := MedianSE(x, 4000, 11, 0)
	naive, _ := MedianSENaive(x, 4000, 11)
	for name, r := range map[string]Result{"fast": fast, "naive": naive} {
		if rel := math.Abs(r.SE-want) / want; rel > 0.20 {
			t.Errorf("%s: SE %.4f vs theory %.4f (%.0f%% off)", name, r.SE, want, 100*rel)
		}
		if r.Median < r.CILower || r.Median > r.CIUpper {
			t.Errorf("%s: median %.3f outside CI [%.3f, %.3f]", name, r.Median, r.CILower, r.CIUpper)
		}
	}
	// The two implementations use different random streams, so they agree
	// only up to Monte Carlo error.
	if rel := math.Abs(fast.SE-naive.SE) / naive.SE; rel > 0.10 {
		t.Errorf("fast SE %.4f and naive SE %.4f differ by %.0f%%", fast.SE, naive.SE, 100*rel)
	}
}

// Every bootstrap median is an observed value (odd n), and the resampled
// indices cover the data without favouring either end.
func TestReplicatesAreDataValues(t *testing.T) {
	x := normalData(101, 5)
	sorted := append([]float64(nil), x...)
	sort.Float64s(sorted)
	r, _ := MedianSE(x, 500, 9, 2)
	for _, v := range r.Replicates {
		i := sort.SearchFloat64s(sorted, v)
		if i >= len(sorted) || sorted[i] != v {
			t.Fatalf("replicate %v is not an observed value", v)
		}
	}
	if math.Abs(r.Bias) > r.SE {
		t.Errorf("bias %.4f larger than SE %.4f", r.Bias, r.SE)
	}
}

// ---- benchmarks: go test -bench . -benchmem ./bootstrap ----

const benchB = 256

func BenchmarkMedianSort(b *testing.B) {
	x := normalData(10_000, 1)
	buf := make([]float64, len(x))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(buf, x)
		medianSort(buf)
	}
}

func BenchmarkMedianQuickselect(b *testing.B) {
	x := normalData(10_000, 1)
	buf := make([]float64, len(x))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(buf, x)
		medianInPlace(buf)
	}
}

func BenchmarkBootstrapNaive(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		x := normalData(n, 1)
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := MedianSENaive(x, benchB, 1); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkBootstrapFast(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		x := normalData(n, 1)
		for _, w := range []int{1, runtime.GOMAXPROCS(0)} {
			b.Run(fmt.Sprintf("n=%d/workers=%d", n, w), func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, err := MedianSE(x, benchB, 1, w); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
