# Design: Separate Section Display for Interface Definitions

Date: 2026-06-24

## Problem

The ISP analyzer scores both structs and interface definitions. Every other
analyzer (SRP/OCP/LSP/DIP) iterates over `pkg.Structs` only, so an interface
definition receives a score for exactly one principle: ISP.

When interfaces and structs share a single Total ranking, interfaces are
structurally advantaged: `Total` is the weighted average over *evaluated*
principles only (`scorer.computeTotal`), so an interface scored solely on ISP
has `Total == ISP`. A small, idiomatic interface gets ISP=100 → Total=100 and
sorts above well-designed structs that are scored across all five principles.

Concrete example (logrus): the `Formatter` interface (ISP=100 → Total=100)
ranks above the `Entry` struct (Total=52.1, evaluated on all five principles).
These are not comparable quantities, yet the table presents them as if they
were.

A prior fix (commit `edc84c7`) made unevaluated principles render as `-` /
JSON `null` instead of a misleading `0.0`. That removed the "all zeros yet
Total 100" contradiction but left the cross-category ranking problem: a
one-principle Total and a five-principle Total still share one sorted column.

## Goal

Present interfaces and structs in **separate sections** so that each Total is
only ever compared against like-for-like, and interface Totals are explicitly
framed as ISP scores. Do not change any scoring logic.

## Non-Goals

- Changing how ISP (or any principle) scores interfaces or structs.
- Changing `computeTotal`. Its "average over evaluated principles" behavior is
  correct and already consistent with the N/A treatment.
- Scoring interfaces on principles other than ISP.

## Architecture

### 1. Carry target kind through the pipeline

`analyzer.Result` and `scorer.ScoreResult` currently have no field
distinguishing an interface target from a struct target. Add one:

- `analyzer.Result` gains `TargetIsInterface bool`.
- `scorer.ScoreResult` gains `IsInterface bool`.

The ISP analyzer is the **single authority** for this flag: it is the only
analyzer that visits interface definitions. `analyzeInterface` sets
`TargetIsInterface = true`; `analyzeStruct` leaves it `false` (zero value).

`scorer.Score` propagates the flag onto the merged `ScoreResult`. Because the
merge key is `pkgPath + "." + name` and Go forbids a struct and an interface
sharing a name in one package, there is no ID collision: a given
`ScoreResult` is unambiguously one kind. When the result is first created
from any analyzer's `Result`, set `sr.IsInterface = r.TargetIsInterface`;
since only ISP sets it true and only ISP ever produces interface targets,
this is stable regardless of analyzer ordering.

### 2. Text formatter: two sections

`TextFormatter.Format` partitions results into structs and interfaces, then
renders:

- A **Structs** table: the existing five-principle columns + Total, with a
  per-section Average. Unchanged from today except it only contains structs.
- An **Interfaces** table: a slim table with just `ISP` and `Total` columns
  (Total == ISP for interfaces), plus a per-section Average. This makes the
  single-principle nature explicit and avoids a row of four `-` columns.

If a section is empty, omit it entirely. Sorting within each section is by
Total ascending, as today.

### 3. JSON formatter: add `is_interface`

Add `IsInterface bool` to `JSONResult` as `"is_interface"`. Existing
per-principle fields keep their current behavior (unevaluated → `null` from
the prior fix). This is additive and backward compatible. The JSON output
remains a single flat `results` array — consumers that want sectioning can
filter on `is_interface` — so no structural break.

## Data Flow

```
ISPAnalyzer.analyzeInterface  -> Result{TargetIsInterface: true}
ISPAnalyzer.analyzeStruct     -> Result{TargetIsInterface: false}
SRP/OCP/LSP/DIP analyzers     -> Result{TargetIsInterface: false}
                                   (only ever produce struct targets)
        |
        v
scorer.Score: merge by targetID, set ScoreResult.IsInterface
        |
        v
TextFormatter: partition by IsInterface -> Structs table + Interfaces table
JSONFormatter: emit is_interface per result (flat array)
```

## Testing

- `scorer`: a package containing one struct and one fat interface yields two
  `ScoreResult`s, one with `IsInterface == true` and one `false`.
- `analyzer/isp`: `analyzeInterface` results carry `TargetIsInterface == true`;
  `analyzeStruct` results carry `false`.
- `formatter/text`: output with both kinds contains both a "Structs" and an
  "Interfaces" header; interface-only and struct-only inputs omit the other
  section; interface rows show ISP and Total only.
- `formatter/json`: `is_interface` is `true` for interface targets, `false`
  for structs; unevaluated principles remain `null`.
- Manual: re-run against logrus/gin/cobra and confirm interfaces appear in
  their own section, no longer interleaved with structs in the main ranking.

## Backward Compatibility

- JSON: at the time of this design, the change was additive only (`is_interface`
  added; the flat `results` array preserved).
  **Update (follow-up commit):** a later fix to the JSON `summary` block changed
  the semantics of two existing fields. `total_structs` and `average_score` now
  count and average **structs only** (previously: all targets, structs and
  interfaces blended). New fields `total_interfaces` and
  `interface_average_score` were added. The `results` array is still unchanged,
  so `differ`/`eval` baselines are unaffected (they do not read `summary`), but
  a consumer that reads `summary.average_score` on a codebase containing
  interfaces will see a different value than before. This was a deliberate
  correctness fix — blending struct (five-principle) and interface (ISP-only)
  Totals is meaningless — not a pure-additive change.
- Text: human-facing format changes (two sections). Acceptable — it is a
  display surface, not a contract. Any golden-file tests are updated to match.
- Diff command (`differ`): operates on `TargetID`, which is unchanged, so
  cross-run diffing is unaffected.
