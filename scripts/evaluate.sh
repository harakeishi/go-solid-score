#!/usr/bin/env bash
#
# evaluate.sh — recall/precision regression gate for go-solid-score.
#
# Runs `gss evaluate` over the annotated testdata packages and checks the
# per-principle confusion matrix against the committed baseline
# (testdata/eval_baseline.json). It fails (exit 1) when a principle regresses:
# a previously-caught violation is now missed (recall floor breached) or a sound
# type is newly flagged (a new false positive). This is the accuracy counterpart
# to scripts/benchmark.sh, which calibrates precision against real-world OSS but
# cannot measure recall (that code has no ground-truth labels).
#
# The testdata packages are listed explicitly on purpose: the go tool excludes
# directories named "testdata" from a "./..." glob, so a glob would silently
# match nothing and the gate would pass having measured nothing.
#
# Usage:
#   scripts/evaluate.sh                 # run the gate against the committed baseline
#   scripts/evaluate.sh --update        # regenerate the baseline (after an intended change)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASELINE="$REPO_ROOT/testdata/eval_baseline.json"

# The annotated ground-truth packages. Keep in sync with the testdata layout.
PKGS=(
	./testdata/srp
	./testdata/ocp
	./testdata/lsp
	./testdata/isp
	./testdata/dip
)

cd "$REPO_ROOT"

if [ "${1:-}" = "--update" ]; then
	echo "regenerating baseline -> $BASELINE"
	go run . evaluate -f json "${PKGS[@]}" >"$BASELINE"
	echo "done. review the diff before committing."
	exit 0
fi

# Print the human-readable table first, then run the regression gate.
go run . evaluate "${PKGS[@]}"
echo
go run . evaluate -f json --baseline "$BASELINE" --fail-on-regression "${PKGS[@]}" >/dev/null
echo "accuracy gate: no regressions against $(basename "$BASELINE")"
