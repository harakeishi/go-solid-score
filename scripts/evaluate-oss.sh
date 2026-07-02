#!/usr/bin/env bash
#
# evaluate-oss.sh — measured accuracy gate against real-world OSS.
#
# Runs `gss evaluate` over a pinned corpus of well-known Go libraries using the
# hand-written ground-truth labels in testdata/oss/labels/, and checks each
# repo's per-principle confusion matrix against its committed baseline in
# testdata/oss/baselines/. It fails (exit 1) when a repo regresses: a known
# real-world violation is no longer caught (recall floor / vanished label) or a
# sound real-world type is newly flagged (a new false positive).
#
# This is the measured evolution of scripts/benchmark.sh: benchmark.sh eyeballs
# mean scores under the assumption "good libraries score well"; here that
# assumption is turned into explicit per-type labels so both error directions
# are counted and gated. See testdata/oss/README.md for the labeling policy.
#
# The corpus is NOT cloned. testdata/oss/corpus is a Go module that pins the
# libraries in go.mod/go.sum; `gss evaluate` runs from that directory with
# import-path patterns, which analyses the pinned release source out of the
# module cache. The only network dependency is the Go module proxy, and go.sum
# checksums everything.
#
# Usage:
#   scripts/evaluate-oss.sh                 # gate every repo against its baseline
#   scripts/evaluate-oss.sh logrus gin      # subset by repo name
#   scripts/evaluate-oss.sh --update        # regenerate all baselines (after an
#                                           # intended change; review the diff)
#   scripts/evaluate-oss.sh --update logrus # regenerate a subset
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CORPUS_DIR="$REPO_ROOT/testdata/oss/corpus"
LABELS_DIR="$REPO_ROOT/testdata/oss/labels"
BASELINE_DIR="$REPO_ROOT/testdata/oss/baselines"

# repo name<TAB>package patterns (space-separated). The name keys the label and
# baseline files; keep all three in sync.
CORPUS=(
	"cobra	github.com/spf13/cobra"
	"logrus	github.com/sirupsen/logrus"
	"zap	go.uber.org/zap go.uber.org/zap/zapcore"
	"gin	github.com/gin-gonic/gin"
	"fasthttp	github.com/valyala/fasthttp"
	"bbolt	go.etcd.io/bbolt"
)

UPDATE=0
if [ "${1:-}" = "--update" ]; then
	UPDATE=1
	shift
fi
ONLY=("$@")

want() { # want <name>: is this repo selected?
	[ ${#ONLY[@]} -eq 0 ] && return 0
	local n
	for n in "${ONLY[@]}"; do
		[ "$n" = "$1" ] && return 0
	done
	return 1
}

BIN="$(mktemp -d)/gss"
echo "building go-solid-score -> $BIN"
(cd "$REPO_ROOT" && go build -o "$BIN" .)

cd "$CORPUS_DIR"
echo "downloading pinned corpus modules (checksummed by go.sum)"
go mod download

fail=0
for entry in "${CORPUS[@]}"; do
	name="${entry%%	*}"
	patterns="${entry#*	}"
	want "$name" || continue

	labels="$LABELS_DIR/$name.yaml"
	baseline="$BASELINE_DIR/$name.json"

	if [ "$UPDATE" -eq 1 ]; then
		echo "== $name: regenerating baseline -> $baseline"
		# shellcheck disable=SC2086 # patterns is a deliberate word-split list
		"$BIN" evaluate -f json --labels "$labels" $patterns >"$baseline"
		continue
	fi

	echo "== $name"
	# shellcheck disable=SC2086
	"$BIN" evaluate --labels "$labels" $patterns
	# shellcheck disable=SC2086
	if ! "$BIN" evaluate -f json --labels "$labels" --baseline "$baseline" \
		--fail-on-regression $patterns >/dev/null; then
		echo "!! $name regressed against $(basename "$baseline")" >&2
		fail=1
	fi
	echo
done

if [ "$UPDATE" -eq 1 ]; then
	echo "done. review the baseline diffs before committing."
	exit 0
fi
if [ "$fail" -ne 0 ]; then
	echo "OSS accuracy gate: regressions detected" >&2
	exit 1
fi
echo "OSS accuracy gate: no regressions against committed baselines"
