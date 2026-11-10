package main

import (
	"strings"
	"testing"

	"github.com/CHbobray/go-bootstrap-median/bootstrap"
)

func TestWriteListingMatchesRLayout(t *testing.T) {
	cfg := bootstrap.StudyConfig{PopMean: 100, PopSD: 10, B: 100}
	rows := []bootstrap.StudyRow{{N: 25, SEMeanCLT: 2, SEMeanSample: 1.99, SEMeanBoot: 1.98, SEMedianBoot: 2.32}}
	var out strings.Builder
	writeListing(&out, cfg, rows)
	for _, want := range []string{
		"Population mean: 100 SD: 10",
		"Estimated standard errors using 100 bootstrap samples",
		"Samples of size n = 25",
		"SE Mean from Central Limit Theorem: 2.00",
		"SE Median from Bootstrap Samples: 2.32",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("listing is missing %q", want)
		}
	}
}
