# Bootstrap standard error of the median: R versus Go

**Question from the consultancy:** how easy is it to use Go in place of R for
computer-intensive statistics, and how much cloud spend would it save?

**Short answer:** for this method, a tuned Go program gave the same estimates
as R's `boot` package, ran about 5 times faster on one core and about 10 times
faster on two, and used about 7 MB of memory against 1.2 GB. On time-billed
virtual machines that is roughly an **80% saving** in compute cost at equal
core count. A first, literal Go translation was *slower* than R, so the saving
only appeared after benchmarking and profiling. A Go version of the course
jump-start Monte Carlo study ran in 0.13 s against 8.6 s for the R original.

## Method

Gelman and Vehtari (2021), "What Are the Most Important Statistical Ideas of
the Past 50 Years?" (*JASA* 116:536), list bootstrapping and simulation-based
inference among their eight ideas. This project uses the nonparametric bootstrap
estimate of the standard error of the median. The median has no simple
distribution-free standard error formula, so resampling is the standard
approach, and the cost grows with (sample size × replicates), which makes it a
fair test of raw computing speed.

Procedure: draw B resamples of size n with replacement, take the median of
each, and report the standard deviation of the B medians. The programs also
report bias and a 95% percentile interval.

## Finding packages

**R.** A search of CRAN and R-universe
(https://r-universe.dev/search/?q=bootstrap) turns up two classic packages:

- **boot** (Canty and Ripley), the functions from Davison and Hinkley (1997):
  https://cran.r-project.org/package=boot
- **bootstrap** (Tibshirani and Leisch), the functions from Efron and
  Tibshirani (1993): https://cran.r-project.org/package=bootstrap

This project uses **boot**, the one that ships with R as a recommended
package. There are three R programs:

- `r/bootstrap_median.R` calls `boot::boot()`. About ten lines.
- `r/bootstrap_loop.R` is a package-free baseline,
  `replicate(B, median(sample(x, replace = TRUE)))`.
- `r/run-bootstrap-median.R` is the course jump-start program by Thomas W.
  Miller, included unchanged, with its sample listing.

**Go.** The places to look are Awesome Go (https://awesome-go.com) and
pkg.go.dev. Go has good numerical building blocks, notably Gonum
(https://pkg.go.dev/gonum.org/v1/gonum/stat), but no package was found that
plays the role `boot` plays in R. The bootstrap here is written on the
standard library, with one third-party dependency:

- https://github.com/seehuhn/mt19937 — the 64-bit Mersenne Twister, the same
  generator family R and Python use by default.

Three things to know about this package:

- It implements the 64-bit variant (MT19937-64). R and Python use the 32-bit
  MT19937, so the same seed does not give the same numbers in Go as in R.
  Results are compared statistically, not draw for draw.
- A generator is not safe for use by several goroutines at once. The tuned
  bootstrap gives each goroutine its own generator.
- It is licensed under GPL-3.0 or later. A firm that distributes software
  built on it would need to release that software under compatible terms.
  Programs run only in-house are not affected.

This is the main ease-of-use gap: in R the method is a function call, in Go it
is a few hundred lines that you write, test and own.

## Repository layout

| Path | Role |
|---|---|
| `bootstrap/bootstrap.go` | reusable package: `Median`, `MedianSENaive` (literal port), `MedianSE` (tuned) |
| `bootstrap/study.go` | reusable package: `Mean`, `StdDev`, `RunStudy` (the jump-start Monte Carlo study) |
| `bootstrap/*_test.go` | unit tests and benchmarks |
| `cmd/bootmedian` | application: bootstrap SE of the median for a CSV file, with logging and profiling |
| `cmd/bootstudy` | application: Go version of `run-bootstrap-median.R`, prints the same listing |
| `cmd/gendata` | application: writes the three test data sets with the Mersenne Twister |
| `r/` | the three R programs and the jump-start sample listing |
| `data/` | the three test data sets, 100,000 observations each |
| `scripts/run_comparison.sh` | runs R and Go on the same files, records time and peak memory |
| `results/` | benchmark output, comparison results, pprof profiles (binary, plus `profiles.txt` as readable text), program log, Go study listing |

## Building the applications

Requires Go 1.24 or later (https://go.dev/dl). From the repository root:

**Windows** (PowerShell or Command Prompt) creates `.exe` files:

```
go build -o bootmedian.exe ./cmd/bootmedian
go build -o bootstudy.exe ./cmd/bootstudy
go build -o gendata.exe ./cmd/gendata
```

**macOS or Linux** creates command-line executables:

```sh
go build -o bootmedian ./cmd/bootmedian
go build -o bootstudy ./cmd/bootstudy
go build -o gendata ./cmd/gendata
```

These are terminal programs, so on macOS they are plain executables rather
than `.app` bundles. To build for another system from any machine, set
`GOOS` and `GOARCH`, for example
`GOOS=windows GOARCH=amd64 go build -o bootmedian.exe ./cmd/bootmedian`.
The Windows build was built and run on Windows 11 (12 cores); the macOS build
was checked by cross-compiling. Benchmark figures in this README are from a
2-vCPU Linux VM.

## Using the applications

```sh
./bootmedian -data data/symmetric_n100000.csv            # 2,000 replicates, all cores
./bootmedian -data data/positive_skew_n100000.csv -b 5000 -seed 7 -workers 4
./bootstudy                                              # jump-start study, B = 100
./bootstudy -b 1000
./gendata -n 100000 -dir data                            # regenerate the test data
```

`bootmedian` reads a CSV with one number per line (a header line is allowed)
and prints a short summary:

```
$ ./bootmedian -data data/symmetric_n100000.csv
Bootstrap standard error of the median

  Data file:       data/symmetric_n100000.csv
  Observations:    100000
  Replicates:      2000 (seed 1)
  Method:          fast, 2 worker(s)

  Median:          49.9818
  Standard error:  0.0408
  Bias:            -0.0014
  95% interval:    49.9015 to 50.0635

  Time:            1.78 s bootstrap, 0.02 s reading data
  Memory:          8.6 MB allocated, 2 garbage collections
```

Options:

| Flag | Default | Meaning |
|---|---|---|
| `-data` | required | one-column CSV file |
| `-b` | 2000 | bootstrap replicates, at least 2 |
| `-seed` | 1 | Mersenne Twister seed |
| `-workers` | 0 | goroutines; 0 means all cores |
| `-impl` | fast | `fast` (tuned) or `naive` (literal port) |
| `-json` | off | print the result as one line of JSON, for scripts |
| `-v` | off | show progress logs on the screen |
| `-log` | none | append JSON log lines to this file |
| `-cpuprofile`, `-memprofile` | none | write pprof profiles |

All three programs check their input before any work starts and report
problems in plain language. For example:

```
$ ./bootmedian -data nope.csv
bootmedian: cannot find data file "nope.csv". Check the path, or create the test data with gendata.

$ ./bootmedian -data data/symmetric_n100000.csv -b 1
bootmedian: -b is 1, but at least 2 bootstrap replicates are needed (1000 or more is typical).

$ ./gendata -n -5
gendata: -n is -5, but each data set needs at least 1 observation (100 or more for a useful bootstrap).
```

A file with a non-numeric line is rejected with the line number and the
offending text.

## Testing and reproducing the results

```sh
go test -cover ./...                                   # unit tests
go test -run xxx -bench . -benchmem ./bootstrap        # benchmarks
./scripts/run_comparison.sh                            # R vs Go; needs R, bash, GNU time
./bootmedian -data data/symmetric_n100000.csv -b 500 \
    -cpuprofile cpu.prof -memprofile mem.prof -log run.log
go tool pprof -top bootmedian cpu.prof                 # read the profile
Rscript r/bootstrap_median.R data/symmetric_n100000.csv 2000 1 1
Rscript r/run-bootstrap-median.R
```

The R scripts need R with the `boot` package (included in standard R
installations). `run_comparison.sh` is a bash script for Linux or macOS.

## Test data

`cmd/gendata` wrote the three files in `data/`, 100,000 observations each, from a
seeded Mersenne Twister, with values written with full float64 precision so R and Go read
identical numbers.

| File | Distribution | Sample skewness |
|---|---|---|
| `positive_skew` | log-normal, exp(N(3.5, 0.6)) | +2.32 |
| `symmetric` | N(50, 10) | +0.01 |
| `negative_skew` | 150 − log-normal | −2.51 |

## Building the Go version

### Testing

`go test` covers 99% of statements in the `bootstrap` package, and each of
the three commands has its own tests. The tests
use synthetic data from the Mersenne Twister and check:

- the median against hand-worked cases, and that the input is not modified;
- quickselect against a full sort on 2,000 random inputs with many ties;
- the percentile function against R's `quantile(type = 7)`;
- that a given seed gives bit-identical replicates for 1, 2, 3 or 8 workers;
- the standard error against the large-sample result for normal data,
  1.2533·σ/√n;
- error handling for empty data, NaN, and fewer than two replicates;
- the Monte Carlo study against the Central Limit Theorem, and that it is
  reproducible;
- the command-line input checks, the CSV reader, both output formats, and a
  full end-to-end run with a log file;
- that `gendata` writes the right number of rows and identical files for the
  same seed;
- that `bootstudy` prints the layout of the R listing.

Testing against R caught a real mistake. The first parallel design seeded
chunk *c* with `seed + c`, so runs with seeds 1, 2, 3 reused almost all the
same random streams and their estimates were nearly identical instead of
independent. Chunks are now seeded with the pair `(seed, c)`.

### Benchmarking and profiling: what improved performance

The first Go version was a literal port of the R idiom: allocate a new slice per replicate,
fill it with `rng.Intn`, sort it, read the middle. It was slower than R,
because R's `median()` uses a partial sort and Go's version did a full one.

CPU profile of the naive version (`results/cpu_naive.prof`, readable copy in
`results/profiles.txt`): about 85% of
time inside `slices.pdqsort` and its helpers. Allocation profile: about 390 MB
allocated and 127 garbage collections for 500 replicates. That pointed to
four changes:

1. **Quickselect instead of a full sort.** The median needs one order
   statistic, not a sorted array. Micro-benchmark at n = 10,000: 760 µs for
   sort, 96 µs for quickselect.
2. **One reusable buffer per worker.** Allocation per run at n = 100,000
   fell from 206 MB to 1.6 MB (benchmark `B/op`), and garbage collections from
   127 to 3.
3. **Cheaper bounded random integers.** A 64-bit multiply-shift replaces
   `rand.Intn`, removing a layer of interface calls and a modulo.
4. **Goroutines.** Replicates are split into chunks of 64, each with its own
   seeded generator, pulled by workers from an atomic counter. No locks, and
   the answer does not depend on the worker count.

After tuning, the profile is about 60% quickselect and most of the rest random
number generation, which is the actual work of the method.

Benchmarks (`results/benchmarks.txt`, 256 replicates per op, 2-vCPU Xeon):

| n | Naive | Tuned, 1 worker | Tuned, 2 workers | Speed-up (2 workers) |
|---|---|---|---|---|
| 1,000 | 17.2 ms | 4.2 ms | 2.3 ms | 7× |
| 10,000 | 229 ms | 39.6 ms | 23.8 ms | 10× |
| 100,000 | 3,053 ms | 418 ms | 229 ms | 13× |

### Logging

`cmd/bootmedian` uses the standard library's `log/slog` with JSON output,
shown on screen with `-v` and appended to a file with `-log`. Each run records the file, n, load time,
implementation, workers, estimates, elapsed time, total allocation and GC
count. `results/run.log` has an example. The logs recorded the effect of each
tuning step.

### Experience

Go's tooling was the easy part: tests, benchmarks, profiler and formatter all
ship with the compiler and needed no setup. The hard part was that nothing
statistical is given. The median, the quantile definition and the parallel
random number scheme each had to be implemented and verified, where R
provides them already checked.

## Monte Carlo benchmark: the jump-start study in R and Go

`r/run-bootstrap-median.R` draws 100 samples at each of four sizes from
N(100, 10), bootstraps each one B times, and compares standard errors of the
mean and median with the Central Limit Theorem. `cmd/bootstudy` does the same
in Go and prints the same listing (`results/listing-from-100-bootstudy-go.txt`).

Estimates with B = 100. The two programs draw different random samples, and
with 100 iterations each estimate carries about 7% sampling error, so they
agree with each other and with theory only to that precision:

| n | CLT SE mean | SE mean from samples, R / Go | SE mean bootstrap, R / Go | SE median bootstrap, R / Go |
|---|---|---|---|---|
| 25 | 2.00 | 1.99 / 1.94 | 1.99 / 1.92 | 2.32 / 2.12 |
| 100 | 1.00 | 1.05 / 1.20 | 1.05 / 1.20 | 1.23 / 1.38 |
| 225 | 0.67 | 0.68 / 0.60 | 0.69 / 0.61 | 0.80 / 0.75 |
| 400 | 0.50 | 0.53 / 0.42 | 0.52 / 0.43 | 0.65 / 0.61 |

With 5,000 iterations the Go median figures settle at 2.31, 1.21, 0.82 and
0.61, close to the large-sample value 1.2533·σ/√n.

| B | R wall time | Go wall time | R peak memory | Go peak memory |
|---|---|---|---|---|
| 100 | 8.6 s | 0.13 s | 67 MB | 3 MB |
| 1,000 | 89.1 s | 1.24 s | 67 MB | 3 MB |

Go is about 65 to 70 times faster here. Most of that gap comes from how the
jump-start R code is written: it grows a data frame with `rbind` inside the
inner loop, which is slow in R. It is teaching code, not tuned code, so this
ratio overstates the language difference. The comparison in the next section
uses R's compiled `boot` package and is the fairer measure.

## R versus Go on the same data

n = 100,000, B = 2,000, same CSV files, 2-vCPU Intel Xeon VM, R 4.3.3 with
boot 1.3-30, Go 1.24. Wall time and peak resident memory from GNU `time`.
Full output is in `results/comparison.jsonl`.

**Results are comparable.** The sample medians are identical. The two
languages use different random streams, so standard errors agree to within
Monte Carlo error (about ±1.6% of the SE at B = 2,000):

| Data | Median | SE, R boot | SE, Go | 95% interval, R | 95% interval, Go |
|---|---|---|---|---|---|
| positive skew | 33.0816 | 0.0758 | 0.0740 | 32.943 – 33.231 | 32.938 – 33.220 |
| symmetric | 49.9818 | 0.0406 | 0.0408 | 49.902 – 50.062 | 49.902 – 50.063 |
| negative skew | 116.9078 | 0.0795 | 0.0810 | 116.746 – 117.057 | 116.747 – 117.066 |

Six further seeds per language on the skewed data gave 0.0738–0.0756 for R
and 0.0740–0.0763 for Go.

**Time and memory** (median of the three data sets):

| Implementation | Wall time | Peak memory | Speed vs R boot |
|---|---|---|---|
| R `boot`, 1 core | 16.4 s | 1,173 MB | 1.0× |
| R `boot`, 2 cores (`multicore`) | 18.7 s | 1,167 MB | 0.9× |
| R hand-written loop, 1 core | 11.9 s | 117 MB | 1.4× |
| Go naive, 1 core | 21.4 s | 9 MB | 0.8× |
| Go tuned, 1 core | 3.3 s | 7 MB | 5.0× |
| Go tuned, 2 cores | 1.6 s | 7 MB | 10.2× |

Notes on these numbers:

- `boot` builds the full B × n index matrix before it starts (2,000 × 100,000
  integers, 800 MB), which is where its memory goes. Memory grows with B × n
  in `boot` and with n × workers in Go.
- `boot`'s forked parallel mode did not help on two cores at this size.
- Timings on this shared VM varied by up to about 20% between runs. The
  ratios were stable: three full runs all gave about 5× and 10×.

## Recommendation

**Use Go in place of R for this method when the job is large, repeated, and
stable.** That means resampling work that runs for hours rather than seconds,
runs on a schedule or inside a service, and uses a method the firm already
understands well enough to specify and test. Under those conditions Go pays
back in three ways: lower run time per core, near-linear use of extra cores,
and small memory, which lets the firm rent smaller machines.

**Stay with R** for exploratory analysis, one-off studies, small data, and any
method where the firm would be re-implementing a mature CRAN package. For a
ten-second job the analyst's time dominates, and R's version here was ten
lines against a few hundred lines of Go plus tests. A sensible middle path is to
prototype in R, keep R as the reference implementation, and port only the
proven hot spots to Go, checking Go against R as this repository does.

### Cloud cost

Provider: **Amazon Web Services EC2**, compute-optimized `c7i` family,
us-east-1, Linux, on-demand. Listed rates at the time of writing (November
2026, from third-party price trackers; confirm at
https://aws.amazon.com/ec2/pricing/on-demand/):

| Instance | vCPU | Memory | On-demand |
|---|---|---|---|
| c7i.large | 2 | 4 GiB | $0.0892 / hour |
| c7i.2xlarge | 8 | 16 GiB | $0.357 / hour |

EC2 bills by time, so at a fixed instance type cost is proportional to run
time. Cost of 1,000 bootstrap jobs of this size on a c7i.large, using the
measured times:

| Implementation | Hours | Cost | Saving vs R boot |
|---|---|---|---|
| R `boot`, 1 core | 4.56 | $0.41 | — |
| R hand-written loop, 1 core | 3.31 | $0.29 | 28% |
| Go tuned, 1 core | 0.92 | $0.082 | 80% |
| Go tuned, 2 cores | 0.45 | $0.040 | 90% |

**Estimated saving: about 80% of compute cost** for this method, comparing
one core of Go with one core of R `boot`. Against the faster hand-written R
loop the saving is about 72%. The 90% figure applies when R leaves the second
core idle; R can also use both cores by running two jobs side by side, so 80%
is the figure to plan on.

Memory adds to this at scale. With n = 1,000,000 and B = 2,000, `boot` would
need about 8 GB for its index matrix alone and so a memory-optimized
instance, while the Go program would need about 8 MB per worker plus the
data.

### Limits of this estimate

- It covers one method. Work that R already hands to compiled code (linear
  algebra, `data.table`, Stan) will show much smaller gaps.
- It was measured on a 2-vCPU machine. Scaling to 8 or more cores is expected
  from the design but was not measured.
- The 80% applies to compute charges for this kind of workload, not to the
  firm's whole cloud bill (storage, data transfer, idle machines), and it
  does not count the developer time to write and maintain Go code.
- R could be sped up too, for example with Rcpp. The comparison is against R as the
  firm uses it today.

## GenAI Tools

This repository was produced with Claude (Anthropic), used as a coding agent
in two prompts: the assignment text, then the grading guidelines and the
jump-start R files.

What the tool did:

- chose the design, wrote the Go package, the three Go commands, the tests
  and benchmarks, the two new R scripts and the comparison script;
- installed R and Go tooling in a cloud Linux workspace and ran everything
  there. All timings, memory figures, profiles and estimates in this README
  come from those runs, on a 2-vCPU VM;
- found and fixed the seeding mistake described under Testing by comparing
  Go results with R across seeds;
- looked up the AWS prices and drafted this README, including the
  recommendation.

What was not generated: `r/run-bootstrap-median.R` and its listing are the
course jump-start files by Thomas W. Miller, unchanged.

Conversation log: _add the share link or exported text file here._

## References

- Gelman, A. and Vehtari, A. (2021). What are the most important statistical
  ideas of the past 50 years? *Journal of the American Statistical
  Association* 116(536), 2087–2097.
- Efron, B. (1979). Bootstrap methods: another look at the jackknife.
  *Annals of Statistics* 7(1), 1–26.
- Davison, A. C. and Hinkley, D. V. (1997). *Bootstrap Methods and Their
  Application*. Cambridge University Press.
- Efron, B. and Tibshirani, R. J. (1993). *An Introduction to the Bootstrap*.
  Chapman & Hall.
- Canty, A. and Ripley, B. boot: Bootstrap R (S-Plus) Functions.
  https://cran.r-project.org/package=boot
- Voss, J. mt19937. https://github.com/seehuhn/mt19937
