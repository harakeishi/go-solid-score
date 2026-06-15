#!/usr/bin/env bash
#
# benchmark.sh — calibration harness for go-solid-score.
#
# Clones a pinned set of well-known, well-reviewed Go libraries, runs the tool
# over each, and prints an aggregate table (mean per-principle scores + the
# lowest-scoring recognisable types). The guiding assumption is that code from
# these libraries is good; when the tool scores their central types
# catastrophically low, the likely cause is a scoring heuristic, not the library.
#
# Use it to regression-check scoring changes against real-world code over time —
# see docs/scoring-analysis.md for the calibration history this reproduces.
#
# Requirements: bash, git, a Go toolchain, network access to clone the corpus,
# and python3 (used only to aggregate the JSON output into the summary table).
#
# Usage:
#   scripts/benchmark.sh                 # full corpus
#   scripts/benchmark.sh cobra gin       # subset by name
#   WORKDIR=/tmp/ssbench scripts/benchmark.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKDIR="${WORKDIR:-/tmp/gss-bench}"
BIN="$WORKDIR/gss"

# repo<TAB>shallow-clone ref. Pinned to a release tag so runs are reproducible.
CORPUS=(
	"spf13/cobra	v1.10.2"
	"gin-gonic/gin	v1.10.0"
	"sirupsen/logrus	v1.9.3"
	"uber-go/zap	v1.27.0"
	"valyala/fasthttp	v1.58.0"
	"etcd-io/bbolt	v1.4.0"
)

mkdir -p "$WORKDIR"
echo "building go-solid-score -> $BIN"
( cd "$REPO_ROOT" && go build -o "$BIN" . )

select_names=("$@")
want() { # want <name> : true if no filter given or name is in the filter
	[ ${#select_names[@]} -eq 0 ] && return 0
	for n in "${select_names[@]}"; do [ "$n" = "$1" ] && return 0; done
	return 1
}

for entry in "${CORPUS[@]}"; do
	repo="${entry%%$'\t'*}"
	ref="${entry##*$'\t'}"
	name="$(basename "$repo")"
	want "$name" || continue
	dir="$WORKDIR/$name"
	if [ ! -d "$dir" ]; then
		echo "cloning $repo@$ref"
		git clone --depth 1 --branch "$ref" --quiet "https://github.com/$repo.git" "$dir"
	fi
	( cd "$dir" && "$BIN" -f json ./... 2>/dev/null ) > "$WORKDIR/$name.json" || true
done

echo
WORKDIR="$WORKDIR" python3 - "${select_names[@]}" <<'PY'
import json, os, sys, glob
wd = os.environ["WORKDIR"]
names = sys.argv[1:]
files = sorted(glob.glob(os.path.join(wd, "*.json")))
print(f"{'library':12} {'targets':>7}  {'SRP':>5} {'OCP':>5} {'LSP':>5} {'ISP':>5} {'DIP':>5} {'total':>6}")
print("-" * 64)
for f in files:
    lib = os.path.splitext(os.path.basename(f))[0]
    if names and lib not in names:
        continue
    try:
        rs = json.load(open(f))["results"]
    except Exception:
        continue
    if not rs:
        continue
    def mean(k): return sum(r[k] for r in rs) / len(rs)
    print(f"{lib:12} {len(rs):7d}  "
          f"{mean('srp'):5.1f} {mean('ocp'):5.1f} {mean('lsp'):5.1f} "
          f"{mean('isp'):5.1f} {mean('dip'):5.1f} {mean('total'):6.1f}")
    low = sorted((r for r in rs), key=lambda r: r["total"])[:3]
    for r in low:
        print(f"    low: {r['name']:28} total={r['total']:5.1f}  "
              f"SRP={r['srp']:.0f} OCP={r['ocp']:.0f} LSP={r['lsp']:.0f} "
              f"ISP={r['isp']:.0f} DIP={r['dip']:.0f}")
PY
