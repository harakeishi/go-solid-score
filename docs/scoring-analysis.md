# Scoring accuracy analysis — benchmarking against real-world Go libraries

This note records a calibration exercise: running `go-solid-score` against
well-known, widely respected Go libraries, analysing where the scores
disagreed with expert judgement, and tightening the analyzers to remove the
systematic false positives that were found.

The guiding assumption: code from libraries like cobra, gin, and logrus is
written by experienced Go developers and has been reviewed by thousands of
users. When our tool scores their central types catastrophically low, the most
likely explanation is a flaw in the scoring heuristic, not in the library.

## Methodology

Three libraries were cloned (shallow) and analysed with `-f json ./...`:

| Library | Targets | Why chosen |
|---------|---------|------------|
| `spf13/cobra` | 9 | The de-facto CLI framework; `Command` is a large, deliberate aggregate type. |
| `gin-gonic/gin` | 56 | High-traffic web framework; `Engine`/`Context` are central facades. |
| `sirupsen/logrus` | 11 | Ubiquitous logger; `Logger`/`Entry`/`*Formatter` exercise config + interface design. |

For each target the per-principle scores were collected and the lowest-scoring,
most-recognisable types were inspected by hand against their source.

## Findings (before the fix)

The headline result: the **most respected types in the ecosystem scored 26–29
out of 100**, driven almost entirely by SRP and DIP.

| Type | total | SRP | OCP | LSP | ISP | DIP |
|------|------:|----:|----:|----:|----:|----:|
| `cobra.Command` | 26.0 | 0 | 100 | 100 | 5 | 0 |
| `gin.Engine` | 29.3 | 0 | 100 | 100 | 5 | 13 |
| `gin.Context` | 28.7 | 0 | 90 | 100 | 5 | 17 |
| `logrus.Logger` | 28.4 | 0 | 100 | 100 | 0 | 13 |

### Root cause 1 — DIP counted things that are not dependencies

The DIP analyzer treated **every** non-whitelisted struct field and parameter
as "a dependency that ought to be an interface". Inspecting `cobra.Command`
showed how wrong this is. Its fields include:

- **Function-typed fields** — `Run func(cmd *Command, args []string)`,
  `PreRunE`, `usageFunc`, … Callbacks/strategies are *behavioural injection*,
  the opposite of a rigid concrete coupling. They were counted as concrete
  dependencies.
- **Self-references** — `parent *Command`, `commands []*Command`,
  `helpCommand *Command`. These model a tree, not an injected collaborator.
- **Value/data containers** — `Annotations map[string]string`. The whitelist
  only stripped a single `*` or `[]` prefix, so maps and channels of primitive
  types leaked through as "concrete dependencies".
- **Named value aliases** — `logrus.FieldMap` (`type FieldMap map[…]…`) is data,
  but as a *named* type it was counted as a concrete dependency, dragging
  `JSONFormatter`/`TextFormatter` DIP to 0.

Because nearly all of a config/aggregate struct's fields fall into these
categories, the interface-to-total ratio collapsed to ~0 — **with high
confidence (0.85)**, which is the worst kind of wrong: confidently incorrect.

### Root cause 2 — DIP penalized method-parameter-only "dependencies"

`logrus.JSONFormatter` implements `Format(entry *Entry) ([]byte, error)`. Its
only non-value "dependency" is the concrete `*Entry` it is *handed* at call
time. DIP scored it 0. But DIP is about the dependencies a type **owns**
(fields / constructor injection); a method parameter is call-time data supplied
by the caller, not an inverted dependency of the receiver.

### Root cause 3 (false negative) — collections of interfaces read as concrete

`IsInterfaceType` only looked at a type's own underlying type, so a field like
`handlers []Handler` (slice of an interface) was classified as **not** an
interface and counted as a concrete dependency — penalizing exactly the kind of
abstraction-oriented design DIP is supposed to reward.

### Observed but not yet changed — SRP/LCOM4 on facade types

`gin.Context`, `cobra.Command`, and `logrus.Logger` score `SRP = 0` with full
confidence. LCOM4 on a 30–60 method facade naturally finds many disconnected
method groups, and the flat `-70` penalty (plus complexity/method-count
penalties) floors the score. Unlike the DIP issues, this is *arguably*
defensible — these types genuinely concentrate many responsibilities, and
`gin.Context` in particular is a frequently-criticised "god object". It is left
unchanged for now and recorded here as future work (see below).

## Changes made

All changes target the DIP false positives, which were the clearest defects.

1. **`IsInterfaceType` now unwraps containers** (pointer/slice/array/map-value/
   channel) to detect an interface element, so `[]Handler` is correctly read as
   an abstraction dependency. (`internal/astutil/helpers.go`)
2. **New `IsFuncType` / `IsValueType` classification**, computed at parse time
   into `FieldInfo`/`ParamInfo`, distinguishing callback fields and value/data
   types (including named aliases) from genuine collaborators.
3. **Whitelist reduces a type to its core element** (`coreTypeName`) so
   `map[string]int`, `chan error`, `[]*time.Time`, etc. are recognised as the
   whitelisted value type they hold.
4. **DIP skips non-dependencies**: function types, self-references, and
   *pure-data* value types — those whose core element is a builtin basic type
   (`map[string]string`, `[]byte`, named aliases like `FieldMap`). A collection
   of a *struct* (e.g. `[]*Worker`) is **not** skipped: its element is a
   concrete collaborator, so it remains a concrete dependency. A collection of
   an *interface* (`[]Handler`) is kept as an abstraction dependency.
   (`analyzer/dip.go`)
5. **DIP treats method parameters as a refinement, not a basis**: when a type
   owns no structural (field/constructor) dependency, DIP is reported as
   *not applicable* instead of penalised to zero.

Genuine concrete couplings are unaffected: the `BadService` fixture
(`db *sql.DB`, `logger *log.Logger`) still scores DIP 0, and a `[]*stage`
collaborator collection is still penalised.

### A note on "not applicable" and the aggregate

A DIP-not-applicable type is returned at the default top score (100) with **low
confidence (0.3)**: owning no concrete dependency vacuously satisfies DIP, and
the low confidence marks the value as not a meaningful signal. This mirrors the
pre-existing treatment of dependency-free structs. The caveat is that the
aggregate `total` does **not** currently weigh by confidence, so such a type
still contributes a high DIP to summaries — and part of the mean-DIP gains below
comes from correcting former false *zeros* (e.g. formatters whose only "input"
is a method parameter) up to this not-applicable 100. A future improvement is to
weigh aggregates by confidence (or expose an explicit N/A marker) so that
not-applicable does not read as a perfect score; see Future work.

## Results (after the fix)

Mean scores across all analysed targets per library:

| Library | DIP before → after | Total before → after |
|---------|-------------------:|---------------------:|
| cobra   | 44.4 → **68.9** | 80.7 → **86.8** |
| gin     | 72.7 → **87.1** | 87.0 → **90.6** |
| logrus  | 28.5 → **73.8** | 68.7 → **80.2** |

Central types, before → after:

| Type | DIP | Total |
|------|----:|------:|
| `cobra.Command` | 0 → 20 | 26.0 → 30.9 |
| `gin.Engine` | 13 → 47 | 29.3 → 37.7 |
| `gin.Context` | 17 → 29 | 28.7 → 31.7 |
| `logrus.Logger` | 13 → 82 | 28.4 → 45.5 |
| `logrus.JSONFormatter` | 0 → 100 (low conf, N/A) | 71.3 → 96.3 |

These numbers are deliberately *more conservative* than an earlier draft that
excluded all slices/maps as data: counting concrete-struct collections (such as
cobra's `[]*Command`/`[]*Group`) as concrete dependencies lowers some scores
again, but avoids the false negative of silently dropping real concrete
collaborators.

The remaining lower totals on `cobra.Command`/`gin.Engine`/`gin.Context` are now
driven partly by genuine concrete coupling and partly by `SRP = 0` (the
facade/LCOM4 effect), which is the next calibration target rather than a DIP
artefact.

## Future work

- **SRP/LCOM4 calibration for large facade types.** Replace the flat `-70`
  LCOM4 penalty with one graduated by the number of connected components
  *relative to* method count, and/or lower confidence when a type is an obvious
  aggregate, so respected facades are not flatly floored at 0.
- **Confidence-aware aggregation / explicit N/A.** The aggregate `total` ignores
  per-principle confidence, so a DIP-not-applicable type (returned at 100 / low
  confidence) reads as a perfect DIP in summaries. Weighing the aggregate by
  confidence, or exposing an explicit "N/A" marker that is excluded from the
  average, would represent "not applicable" more honestly than a top score.
- **ISP confidence for embedded/promoted methods.** Several large public
  surfaces come from interface embedding; verify these are scored via the
  decorator/adapter path rather than as bloated interfaces.
- **Reproducibility.** Consider committing a small benchmark script that clones a
  pinned set of libraries and prints the aggregate table above, so scoring
  changes can be regression-checked against real-world code over time.

## Reproducing

```bash
go build -o /tmp/gss .
for repo in spf13/cobra gin-gonic/gin sirupsen/logrus; do
  name=$(basename "$repo")
  git clone --depth 1 "https://github.com/$repo.git" "/tmp/$name"
  (cd "/tmp/$name" && /tmp/gss -f json ./...) > "/tmp/$name.json"
done
```
