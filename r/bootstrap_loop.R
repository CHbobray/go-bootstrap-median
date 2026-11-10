#!/usr/bin/env Rscript
# Hand-written base-R bootstrap, no packages. Second R baseline: it avoids
# the B x n index matrix that boot::boot builds up front.
# Usage: Rscript r/bootstrap_loop.R <csv> [B] [seed]
args <- commandArgs(trailingOnly = TRUE)
B    <- if (length(args) >= 2) as.integer(args[2]) else 2000L
seed <- if (length(args) >= 3) as.integer(args[3]) else 1L
x <- read.csv(args[1])$x
set.seed(seed)
t <- system.time(m <- replicate(B, median(sample(x, replace = TRUE))))[["elapsed"]]
cat(sprintf("median=%.6f se=%.6f boot_seconds=%.2f\n", median(x), sd(m), t))
