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

## Second validation round (wider corpus)

To confirm the DIP fixes generalise and to probe the *other* principles, a more
diverse set was analysed: `uber-go/zap`, `go-chi/chi`, `etcd-io/bbolt`,
`spf13/viper`, `gorilla/mux`, and `pkg/errors`. The DIP changes held up (no new
DIP false positives), and the SRP/LCOM4 facade effect reappeared as expected
(`bbolt.Bucket`, `bbolt.Tx`, `mux.Route`, `zap.contextObserver`). One new,
clear-cut false positive surfaced in **LSP**.

### LSP penalised idiomatic fail-fast guard panics

`chi.Mux` — a widely used HTTP router — scored **LSP = 20**. Inspection showed
every panic in `Mux` is a fail-fast guard on invalid API usage:

```go
func (mx *Mux) Method(method, pattern string, handler http.Handler) {
    m, ok := methodMap[strings.ToUpper(method)]
    if !ok {
        panic(fmt.Sprintf("chi: '%s' http method is not supported.", method))
    }
    ...
}
```

Panicking on a programming error / violated precondition is idiomatic Go
(`regexp.MustCompile`, `http.ServeMux.Handle` on a duplicate pattern, etc.) and
is *not* a Liskov violation: every conforming implementation rejects the same
invalid input. The real LSP smell is an **unconditional** panic — a method whose
defined behaviour is to abort, e.g. a `panic("not implemented")` stub on a
read-only type.

**Fix:** the LSP analyzer now penalises only *unconditional* panics. A new
`HasUnconditionalPanic` signal (computed in the walker) flags a `panic(...)` on
the method's straight-line path; panics nested inside `if`/`for`/`switch`/
`select` guards are treated as fail-fast and ignored. Because this only relaxes
penalties, no LSP true positive regresses — the `ReadOnlySaver.Save` stub
(unconditional panic) is still flagged. `chi.Mux` LSP went **20 → 100**, and a
guard-panic check on `gin.Redirect.Render` confirmed its remaining penalty comes
from a separate no-op heuristic, not the panic.

### Observed but not changed

- **OCP on dynamic-config code.** `viper.Viper` scores `OCP = 25` from heavy
  type switching on `any` config values — arguably a real OCP cost, left as-is.
- **ISP on externally-mandated wide interfaces.** `zap.jsonEncoder` scores
  `ISP = 0` for implementing `zapcore.Encoder` (~20 `AppendX` methods). The
  width is dictated by a required external contract, not by the implementer; the
  decorator/adapter exemption does not cover this. A candidate refinement is to
  discount ISP when a type's public surface is mandated by an interface it
  implements rather than self-imposed.

## Third validation round (OCP)

A further batch was analysed: `prometheus/client_golang`, `stretchr/testify`,
`urfave/cli`, `gorilla/websocket`, `hashicorp/golang-lru`, and
`json-iterator/go`. The DIP and LSP fixes held. The new signal was in **OCP**.

### OCP penalised interface feature-detection

Several well-regarded types scored low OCP not because they branch on concrete
types, but because they use **comma-ok assertions to an interface** to detect an
optional capability — the canonical extensible pattern:

```go
// prometheus/client_golang promhttp delegators
if p, ok := w.(http.Pusher); ok { ... }   // pusherDelegator
// gorilla/websocket
if d, ok := dialer.(proxy.ContextDialer); ok { ... }
```

Adding a new type that implements the asserted interface requires **no change**
to this code, so it is open for extension — the opposite of an OCP violation.
The analyzer was counting every `x.(T)` the same, regardless of whether `T` is
an interface (feature detection) or a concrete type (a downcast, the real OCP
smell).

**Fix:** the walker now counts only assertions whose target is a *concrete*
type. Interface-target assertions are ignored, and the `x.(type)` form of a type
switch is no longer double-counted alongside the switch itself. As with the LSP
change this only relaxes penalties, so the `Router` fixture (which asserts to
`string`/`int`) still scores low. Mean OCP and notable recoveries:

| Library | OCP mean | Notable recoveries (concrete asserts stay penalised) |
|---------|---------:|------------------------------------------------------|
| client_golang | 97.2 → 99.2 | `pusherDelegator`/`hijackerDelegator` 70 → 100 |
| testify | 90.4 → 94.8 | `Mock` 70 → 100 |
| cli | 93.4 → 96.9 | `FlagBase` 20 → 60 |
| viper | 95.6 → 97.9 | `Viper` 25 → 65 (concrete config type-switches remain) |
| json-iterator | 97.7 → 98.9 | `anyCodec` 70 → 100 |
| websocket | 100 → 100 | unchanged — its asserts are concrete (`*CloseError`) |

`gorilla/websocket` acts as a control: its assertions are downcasts to concrete
types, so its OCP is unchanged — confirming the fix discriminates correctly.

## Fourth validation round (SRP)

A filesystem/util-heavy batch was analysed: `valyala/fasthttp`,
`go-playground/validator`, `spf13/afero`, `BurntSushi/toml`, `robfig/cron`, and
`google/uuid`. DIP/LSP/OCP held. The new signal was an **SRP/LCOM4** false
positive, most visible in `afero` (mean SRP **51**) and on small error types.

### LCOM4 was inflated by stateless methods

`google/uuid`'s `URNPrefixError` scored `SRP = 60`. It has two methods:

```go
func (e URNPrefixError) Error() string { return ... e.prefix ... } // uses a field
func (e URNPrefixError) Is(target error) bool {                    // uses NO field
    _, ok := target.(URNPrefixError); return ok
}
```

LCOM4 connects methods that share a field or call each other. `Is` is a standard
`errors.Is` convention method that only inspects its *argument*, so it shares no
field with `Error` and was counted as a second, disconnected "responsibility" —
LCOM4 = 2, a −40 penalty. `spf13/afero` showed the same effect at scale: stateless
adapters such as `OsFs` (an empty struct that forwards to the `os` package) have
*no fields at all*, so every method was an isolated component and SRP cratered to
~15 — even though a fieldless adapter has no field-cohesion to violate.

**Fix:** a method that accesses no receiver field *and* is uncoupled from
siblings is excluded from the LCOM4 component count. LCOM measures cohesion over
fields, so a stateless method is outside the metric. This generalises a narrower
pre-existing mitigation (which only spared fieldless structs with ≤5 methods).
Genuine low cohesion is unaffected — methods over disjoint fields still
fragment — and oversized types are still caught by the method-count penalty.

Across all 20 libraries analysed so far, this changed **88 targets upward and 0
downward**. Examples: `uuid.URNPrefixError` 60→100, `afero.OsFs` 15→85,
`afero` mean SRP 51→84, json-iterator mean 77→89. Crucially, genuine god-objects
are *not* whitewashed: `gin.Context` 0→65, `gin.Engine` 0→25, `fasthttp.Request`
0→25 — still clearly penalised, just no longer floored at zero by the stateless
methods mixed into their large surface. The `GodStruct` fixture (every method
touches a field) is unchanged at 30.

## Future work

- **SRP/LCOM4 calibration for large facade types.** The stateless-method
  exclusion (fourth round) lifted facades off an absolute 0, but types whose
  *stateful* methods genuinely operate over disjoint fields (e.g. `gin.Engine`,
  `fasthttp.Request`) still take the flat `-70` LCOM4 hit. A remaining
  refinement is to graduate that penalty by the number of connected components
  *relative to* method count, so a 3-of-40 split reads differently from a 3-of-5
  split, and/or to lower confidence for obvious aggregates.
- **ISP for externally-mandated interfaces.** `afero.File`/`Fs` implementers and
  `zap.jsonEncoder` score low ISP for a wide public surface that is dictated by
  a standard interface they must satisfy (os.File-like, `zapcore.Encoder`), not
  by self-imposed bloat. Discounting ISP when a type's surface mirrors an
  interface it implements would reduce these false positives.
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
