# ISP Interface-Target Scoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add interface definitions as a first-class ISP scoring target so fat interface definitions (the principle's true subject) are scored, while keeping the existing struct scoring (and its FatImpl recall guard) intact.

**Architecture:** `ISPAnalyzer.Analyze` already iterates `pkg.Structs`; add a parallel loop over `pkg.Interfaces` that emits one `Result` per interface, scored by `InterfaceInfo.TotalMethods` (mapped to cross the ISP threshold of 50 at >10 methods, matching the interfacebloat linter), with a bonus for embedding-composed interfaces. Interface targets get IDs `<pkgPath>.<name>` via the existing `scorer.targetID`, so they form distinct rows from structs and join to inline `// solid:want` labels through the Phase 1 harness with no new plumbing.

**Tech Stack:** Go, existing `analyzer`/`model`/`eval` packages, `go test`, `scripts/evaluate.sh`.

**Spec:** `docs/superpowers/specs/2026-06-17-isp-interface-target-design.md`

---

## File Structure

- `analyzer/isp.go` — add `analyzeInterface` method + interface loop in `Analyze`. Self-contained; struct path untouched.
- `analyzer/isp_test.go` — add unit tests for interface scoring (fat / small / composed).
- `testdata/isp/bad.go` — label `FatInterface` as a violation.
- `testdata/isp/good.go` — label `Reader`, `Writer`, `ReadWriter` as ok.
- `testdata/eval_baseline.json` — regenerate to record the ISP TP increase.

The branch `feat/isp-interface-target` is already checked out with the design doc committed.

---

## Task 1: Interface scoring in the ISP analyzer

**Files:**
- Modify: `analyzer/isp.go` (add `analyzeInterface`; extend `Analyze`)
- Test: `analyzer/isp_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `analyzer/isp_test.go`:

```go
// TestISPAnalyzer_FatInterface checks that a fat interface definition is scored
// below the ISP threshold (50) as its own target — the principle's true subject.
func TestISPAnalyzer_FatInterface(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/isp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewISPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "FatInterface" {
			if r.Score >= 50 {
				t.Errorf("FatInterface ISP score %.1f should be < 50 (flagged as a violation)", r.Score)
			}
			return
		}
	}
	t.Error("FatInterface not found in results — interfaces are not being scored")
}

// TestISPAnalyzer_SmallInterface checks that a small, focused interface scores
// at the top (Go idiom: io.Reader-style single-method interfaces).
func TestISPAnalyzer_SmallInterface(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/isp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewISPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "Reader" {
			if r.Score < 90 {
				t.Errorf("Reader (1 method) ISP score %.1f should be >= 90", r.Score)
			}
			return
		}
	}
	t.Error("Reader interface not found in results")
}

// TestISPAnalyzer_ComposedInterface checks that an interface composed of small
// role interfaces via embedding is not flagged (it is ISP-faithful).
func TestISPAnalyzer_ComposedInterface(t *testing.T) {
	pkgs, err := parser.Parse([]string{"../testdata/isp"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	a := analyzer.NewISPAnalyzer()
	results := a.Analyze(pkgs[0])

	for _, r := range results {
		if r.TargetName == "ReadWriter" {
			if r.Score < 90 {
				t.Errorf("ReadWriter (composed) ISP score %.1f should be >= 90 (no FP)", r.Score)
			}
			return
		}
	}
	t.Error("ReadWriter interface not found in results")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./analyzer/ -run 'TestISPAnalyzer_(FatInterface|SmallInterface|ComposedInterface)' -v`
Expected: FAIL — all three report "not found in results" (interfaces are not scored yet).

- [ ] **Step 3: Add the interface loop to `Analyze`**

In `analyzer/isp.go`, change the `Analyze` method body so the existing struct loop is followed by an interface loop:

```go
func (a *ISPAnalyzer) Analyze(pkg *model.PackageInfo) []Result {
	var results []Result

	// Structs are scored on their public interface size (an SRP-leaning
	// signal, kept for continuity and as the FatImpl recall guard).
	for _, s := range pkg.Structs {
		results = append(results, a.analyzeStruct(s, pkg))
	}

	// Interface definitions are the principle's true subject: ISP violations
	// live in fat interfaces that force clients to depend on methods they do
	// not use. Only in-package interfaces are scored, so external contracts
	// (afero.File etc.) are structurally excluded.
	for _, iface := range pkg.Interfaces {
		results = append(results, a.analyzeInterface(iface, pkg))
	}

	return results
}
```

- [ ] **Step 4: Add the `analyzeInterface` method**

Append to `analyzer/isp.go`:

```go
// analyzeInterface scores a single interface definition by its total method
// count (including methods promoted from embedded interfaces). The thresholds
// map the interfacebloat linter's de-facto standard — a fat interface is one
// with more than ~10 methods — onto the 0-100 scale, so an 11-method interface
// lands below the ISP pass threshold (50) and is flagged as a violation.
// Interfaces composed of smaller role interfaces via embedding are ISP-faithful
// and receive a bonus to avoid penalizing the idiomatic io.ReadWriteCloser
// composition pattern.
func (a *ISPAnalyzer) analyzeInterface(iface *model.InterfaceInfo, pkg *model.PackageInfo) Result {
	r := Result{
		Principle:  ISP,
		TargetPkg:  pkg.PkgPath,
		TargetName: iface.Name,
		TargetFile: iface.File,
		TargetLine: iface.Line,
		Score:      100,
		Confidence: ConfidenceMedium,
	}

	mc := iface.TotalMethods
	switch {
	case mc <= 3:
		// Good — small, focused interface (Go idiom).
	case mc <= 5:
		r.Score = 90
	case mc <= 7:
		r.Score = 75
		r.Details = append(r.Details, fmt.Sprintf("%d methods (consider splitting)", mc))
	case mc <= 10:
		r.Score = 60
		r.Details = append(r.Details, fmt.Sprintf("%d methods (large interface)", mc))
	case mc <= 15:
		r.Score = 40
		r.Details = append(r.Details, fmt.Sprintf("%d methods (fat interface — clients depend on methods they don't use)", mc))
	default:
		r.Score = 20
		r.Details = append(r.Details, fmt.Sprintf("%d methods (severely bloated interface)", mc))
	}

	// Interfaces composed by embedding small role interfaces are ISP-faithful.
	if len(iface.Embeds) > 0 {
		r.Score += 15
		r.Details = append(r.Details, fmt.Sprintf("composes %d embedded interface(s)", len(iface.Embeds)))
	}

	if mc >= 8 {
		r.Confidence = ConfidenceMediumHigh
	} else if len(iface.Embeds) > 0 {
		r.Confidence = ConfidenceMediumHigh
	}

	r.Score = Clamp(r.Score)
	return r
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./analyzer/ -run 'TestISPAnalyzer_(FatInterface|SmallInterface|ComposedInterface)' -v`
Expected: PASS for all three.

- [ ] **Step 6: Run the full analyzer suite to confirm the struct path is intact**

Run: `go test ./analyzer/ -v -run TestISPAnalyzer`
Expected: PASS — `TestISPAnalyzer_Good` (SimpleReader) and `TestISPAnalyzer_Bad` (FatImpl) still pass; FatImpl is still found and scored < 80.

- [ ] **Step 7: Commit**

```bash
git add analyzer/isp.go analyzer/isp_test.go
git commit -m "$(cat <<'EOF'
ai/feat(analyzer): score interface definitions for ISP

Add analyzeInterface so fat interface definitions — the principle's true
subject — are scored, mapping interfacebloat's >10-method standard onto the
0-100 scale (11 methods -> 40, below the ISP threshold of 50). Embedding-composed
interfaces get a bonus to avoid flagging the idiomatic io.ReadWriteCloser
pattern. Struct scoring is unchanged, so the FatImpl recall guard holds.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Ground-truth labels for interfaces

**Files:**
- Modify: `testdata/isp/bad.go` (label `FatInterface`)
- Modify: `testdata/isp/good.go` (label `Reader`, `Writer`, `ReadWriter`)

- [ ] **Step 1: Label `FatInterface` as a violation**

In `testdata/isp/bad.go`, replace the `FatInterface` doc comment:

```go
// FatInterface forces implementors to depend on methods they don't use.
// solid:want ISP=violation reason="Martin ISP / Pike 'the bigger the interface, the weaker the abstraction': 11 methods force clients to depend on methods they don't use — the interface-definition recall guard"
type FatInterface interface {
```

- [ ] **Step 2: Label the small and composed interfaces as ok**

In `testdata/isp/good.go`, add a `solid:want` line to each of the three interface doc comments:

```go
// Reader is a small, focused interface (Go idiomatic).
// solid:want ISP=ok reason="single-method role interface (io.Reader idiom); minimal client coupling"
type Reader interface {
	Read(p []byte) (n int, err error)
}

// Writer is a small, focused interface.
// solid:want ISP=ok reason="single-method role interface; minimal client coupling"
type Writer interface {
	Write(p []byte) (n int, err error)
}

// ReadWriter composes two small interfaces.
// solid:want ISP=ok reason="composed from small role interfaces via embedding (io.ReadWriter pattern); ISP-faithful"
type ReadWriter interface {
	Reader
	Writer
}
```

- [ ] **Step 3: Verify the labels parse and score as expected via evaluate**

Run: `go run . evaluate -f json ./testdata/isp`
Expected: valid JSON; ISP shows `tp >= 2` (FatImpl struct + FatInterface), `fp == 0`. Confirm with:

```bash
go run . evaluate -f json ./testdata/isp 2>/dev/null | python3 -c "import sys,json; p=json.load(sys.stdin)['per_principle']['ISP']; print('tp=%d fp=%d fn=%d tn=%d'%(p['tp'],p['fp'],p['fn'],p['tn'])); assert p['tp']>=2 and p['fp']==0, p"
```
Expected: prints `tp=2 fp=0 fn=0 tn=...` and exits 0 (no AssertionError).

- [ ] **Step 4: Commit**

```bash
git add testdata/isp/bad.go testdata/isp/good.go
git commit -m "$(cat <<'EOF'
ai/test(eval): label ISP interface targets (FatInterface TP, role interfaces TN)

FatInterface (11 methods) becomes a new true positive now that interfaces are
scored; Reader/Writer/ReadWriter are sound (ok), with the composed ReadWriter
guarding against false positives on the embedding pattern.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Regenerate the accuracy baseline and verify the gate

**Files:**
- Modify: `testdata/eval_baseline.json`

- [ ] **Step 1: Inspect the current ISP confusion before regenerating**

Run: `go run . evaluate ./testdata/srp ./testdata/ocp ./testdata/lsp ./testdata/isp ./testdata/dip`
Expected: the ISP row now reflects the interface targets (recall still 1.0, denominator increased from 1 to 2). Note the numbers for the commit message.

- [ ] **Step 2: Confirm recall is non-regressing against the OLD baseline**

The committed baseline still has the pre-change ISP (tp=1). Run the gate to confirm nothing regressed (TP must not drop, FP must not rise):

Run: `go run . evaluate --baseline testdata/eval_baseline.json ./testdata/srp ./testdata/ocp ./testdata/lsp ./testdata/isp ./testdata/dip`
Expected: no regression printed to stderr (ISP TP went 1->2, an improvement; FP stayed 0). The command exits 0.

- [ ] **Step 3: Regenerate the baseline**

Run: `scripts/evaluate.sh --update`
Then review the diff:

Run: `git diff testdata/eval_baseline.json`
Expected: the ISP block's `tp` and `recall_denominator` increase (1 -> 2); `fp` stays 0. No other principle changes.

- [ ] **Step 4: Confirm the gate passes against the new baseline**

Run: `scripts/evaluate.sh`
Expected: prints the table then `accuracy gate: no regressions against eval_baseline.json`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add testdata/eval_baseline.json
git commit -m "$(cat <<'EOF'
ai/test(eval): update accuracy baseline for ISP interface targets

ISP true positives go 1 -> 2 (FatImpl struct + FatInterface definition) and the
recall denominator grows accordingly; no false positives introduced. Recorded so
the non-regression gate measures against the new, larger ISP basis.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Run the full suite with the race detector**

Run: `go test -race ./...`
Expected: all packages `ok` (ignore "no test files" lines).

- [ ] **Step 2: Format and vet**

Run: `gofmt -l . | grep -v '^$' || echo clean` then `go vet ./...`
Expected: `clean` (no files listed) and no vet output.

- [ ] **Step 3: Confirm the evaluate report end-to-end**

Run: `go run . evaluate ./testdata/srp ./testdata/ocp ./testdata/lsp ./testdata/isp ./testdata/dip`
Expected: ISP row shows precision 1.000, recall 1.000, n(TP+FN)=2; OCP/LSP still carry their known FNs; no principle shows a new FP.

- [ ] **Step 4: Build the binary as a final smoke test**

Run: `go build -o /tmp/gss . && /tmp/gss -f json ./testdata/isp | python3 -c "import sys,json; rows={r['name'] for r in json.load(sys.stdin)['results']}; print('FatInterface' in rows and 'FatImpl' in rows)"`
Expected: prints `True` — both the interface and the struct appear as scored targets.

---

## Self-Review Notes

- **Spec coverage:** §2.1 interface loop (Task 1 Step 3), §2.2 score table (Task 1 Step 4), §2.3 embedding bonus (Task 1 Step 4), §2.4 in-package-only — satisfied implicitly because `pkg.Interfaces` only contains in-package definitions (no extra code needed; verified by the composed-interface FP test). §2.5 confidence (Task 1 Step 4). §3 labels (Task 2). §4 completion (Tasks 3-4). All covered.
- **Struct path untouched:** Task 1 only adds a loop + new method; `analyzeStruct` and its tests are unchanged — the recall guard is verified in Task 1 Step 6.
- **Type consistency:** `analyzeInterface(iface *model.InterfaceInfo, pkg *model.PackageInfo) Result` uses `iface.TotalMethods`, `iface.Embeds`, `iface.File`, `iface.Line` — all confirmed present on `model.InterfaceInfo`. `Clamp`, `ConfidenceMedium`, `ConfidenceMediumHigh` confirmed in `analyzer/analyzer.go`.
