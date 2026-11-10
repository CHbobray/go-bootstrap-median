package bootstrap

import (
	"errors"
	"math"
	"testing"
)

func jumpStartConfig() StudyConfig {
	return StudyConfig{PopMean: 100, PopSD: 10, SampleSizes: []int{25, 100, 225, 400},
		Iterations: 100, B: 100, Seed: 9999}
}

func TestMeanAndStdDev(t *testing.T) {
	x := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	if got := Mean(x); got != 5 {
		t.Errorf("Mean = %v, want 5", got)
	}
	// R: sd(c(2,4,4,4,5,5,7,9)) = 2.13809
	if got := StdDev(x); math.Abs(got-2.138090) > 1e-6 {
		t.Errorf("StdDev = %v, want 2.13809", got)
	}
}

// The study should land near the Central Limit Theorem values, as the R
// listing does. With 100 iterations an SD estimate is good to about 15%.
func TestStudyAgreesWithTheory(t *testing.T) {
	rows, err := RunStudy(jumpStartConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		for name, got := range map[string]float64{"sample": r.SEMeanSample, "boot": r.SEMeanBoot} {
			if rel := math.Abs(got-r.SEMeanCLT) / r.SEMeanCLT; rel > 0.25 {
				t.Errorf("n=%d: SE mean (%s) %.3f vs CLT %.3f", r.N, name, got, r.SEMeanCLT)
			}
		}
		// For a normal population the median is less efficient than the mean.
		if r.SEMedianBoot <= r.SEMeanCLT {
			t.Errorf("n=%d: SE median %.3f not above SE mean %.3f", r.N, r.SEMedianBoot, r.SEMeanCLT)
		}
	}
}

func TestStudyIsReproducible(t *testing.T) {
	a, _ := RunStudy(jumpStartConfig())
	b, _ := RunStudy(jumpStartConfig())
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("row %d differs between identical runs", i)
		}
	}
}

func TestStudyRejectsBadConfig(t *testing.T) {
	bad := []StudyConfig{
		{},
		{PopSD: 1, Iterations: 1, B: 10, SampleSizes: []int{5}},
		{PopSD: 1, Iterations: 10, B: 0, SampleSizes: []int{5}},
		{PopSD: 1, Iterations: 10, B: 10, SampleSizes: []int{0}},
	}
	for i, cfg := range bad {
		if _, err := RunStudy(cfg); !errors.Is(err, ErrStudyConfig) {
			t.Errorf("config %d: got %v, want ErrStudyConfig", i, err)
		}
	}
}

func BenchmarkJumpStartStudy(b *testing.B) {
	cfg := jumpStartConfig()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := RunStudy(cfg); err != nil {
			b.Fatal(err)
		}
	}
}
