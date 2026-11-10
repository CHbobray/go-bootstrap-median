#!/usr/bin/env Rscript
# Bootstrap standard error of the median with the CRAN package 'boot'.
#   https://cran.r-project.org/package=boot
#
# Usage: Rscript r/bootstrap_median.R <csv> [B] [seed] [ncpus]
# Prints one JSON line so the output lines up with the Go program.

suppressPackageStartupMessages(library(boot))

args  <- commandArgs(trailingOnly = TRUE)
file  <- args[1]
B     <- if (length(args) >= 2) as.integer(args[2]) else 2000L
seed  <- if (length(args) >= 3) as.integer(args[3]) else 1L
ncpus <- if (length(args) >= 4) as.integer(args[4]) else 1L

t_read <- system.time(x <- read.csv(file)$x)[["elapsed"]]

# R's default generator is the Mersenne Twister. Forked workers need
# L'Ecuyer streams to stay reproducible.
if (ncpus > 1) RNGkind("L'Ecuyer-CMRG") else RNGkind("Mersenne-Twister")
set.seed(seed)

med_fun <- function(d, i) median(d[i])

t_boot <- system.time(
  bt <- boot(data = x, statistic = med_fun, R = B,
             parallel = if (ncpus > 1) "multicore" else "no", ncpus = ncpus)
)[["elapsed"]]

ci <- quantile(bt$t, c(0.025, 0.975), names = FALSE)  # type 7 percentile interval
mem <- gc()

cat(sprintf(
  paste0('{"n":%d,"b":%d,"median":%.10g,"se":%.10g,"bias":%.10g,',
         '"ci_lower":%.10g,"ci_upper":%.10g,"workers":%d,"file":"%s",',
         '"impl":"R boot","seed":%d,"read_seconds":%.4f,"boot_seconds":%.4f,',
         '"r_peak_heap_mb":%.1f}\n'),
  length(x), B, bt$t0, sd(bt$t), mean(bt$t) - bt$t0, ci[1], ci[2], ncpus, file,
  seed, t_read, t_boot, sum(mem[, 6])))
