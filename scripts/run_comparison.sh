#!/usr/bin/env bash
# Runs R and Go on the same CSV files and records elapsed time and peak
# resident memory (GNU time) for each. Output: results/comparison.jsonl
set -euo pipefail
cd "$(dirname "$0")/.."

N=${N:-100000}
B=${B:-2000}
CORES=$(nproc)
OUT=results/comparison.jsonl
mkdir -p bin results
go build -o bin/gendata ./cmd/gendata
go build -o bin/bootmedian ./cmd/bootmedian
[ -f "data/symmetric_n${N}.csv" ] || ./bin/gendata -n "$N" -dir data
: > "$OUT"

run() { # label, command...
  local label=$1; shift
  local json
  json=$(/usr/bin/time -f '%e %M' -o /tmp/time.out "$@" 2>/dev/null)
  read -r wall rss < /tmp/time.out
  echo "${json%\}},\"label\":\"$label\",\"wall_seconds\":$wall,\"peak_rss_mb\":$(awk "BEGIN{printf \"%.1f\", $rss/1024}")}" >> "$OUT"
  echo "  $label: wall ${wall}s, peak RSS $(awk "BEGIN{printf \"%.0f\", $rss/1024}") MB"
}

for shape in positive_skew symmetric negative_skew; do
  f="data/${shape}_n${N}.csv"
  echo "$f (B=$B)"
  run "R boot, 1 core"            Rscript r/bootstrap_median.R "$f" "$B" 1 1
  run "R boot, $CORES cores"      Rscript r/bootstrap_median.R "$f" "$B" 1 "$CORES"
  run "Go naive, 1 core"          ./bin/bootmedian -data "$f" -b "$B" -impl naive
  run "Go fast, 1 core"           ./bin/bootmedian -data "$f" -b "$B" -impl fast -workers 1
  run "Go fast, $CORES cores"     ./bin/bootmedian -data "$f" -b "$B" -impl fast -workers "$CORES"
done
echo "wrote $OUT"
