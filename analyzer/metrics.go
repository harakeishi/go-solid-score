package analyzer

import (
	"github.com/harakeishi/go-solid-score/model"
	"github.com/harakeishi/go-solid-score/rules"
)

// StructMetrics computes the full set of named metrics for a struct target.
// These are the facts the declarative rule engine reads when scoring; every
// signal the built-in SOLID rules depend on (and that custom rules may
// reference) is produced here, in one place, so the metric vocabulary is
// centralized rather than scattered across per-principle analyzers.
//
// whitelist is the user-provided DIP whitelist; it only affects the dependency
// metrics and is otherwise ignored.
func StructMetrics(s *model.StructInfo, pkg *model.PackageInfo, whitelist []string) rules.Metrics {
	m := rules.Metrics{}

	methods := s.Methods
	pubMethods := s.PublicMethods()
	m["method_count"] = float64(len(methods))
	m["public_method_count"] = float64(len(pubMethods))

	// Fields (named, non-embedded). has_fields drives the SRP stateless guard.
	// ownFields is the set of own named-field names; LSCC counts accesses only
	// to these so external/promoted field reads cannot inflate cohesion.
	namedFields := 0
	ownFields := make(map[string]bool)
	for _, f := range s.Fields {
		if f.Name != "" {
			namedFields++
			ownFields[f.Name] = true
		}
	}
	m["field_count"] = float64(namedFields)
	m["has_fields"] = boolMetric(namedFields > 0)

	// --- SRP: LSCC cohesion, complexity ---
	// cohesion_method_count is the method count after excluding Go convention
	// methods (errors.Is/As/Unwrap); LSCC is only meaningful when it is >= 2, so
	// the cohesion rule guards on it rather than on raw method_count.
	lscc, ownFieldMethodCount := calculateLSCC(methods, ownFields)
	m["lscc"] = lscc
	m["own_field_access_method_count"] = float64(ownFieldMethodCount)
	m["cohesion_method_count"] = float64(len(effectiveCohesionMethods(methods)))
	// total_complexity is WMC (Weighted Methods per Class, Chidamber & Kemerer
	// 1994) with cyclomatic complexity as the method weight. Note WMC correlates
	// strongly with class size (El Emam 2001), which is why the preset rules
	// pair it with size-independent cohesion signals instead of scoring on it
	// alone. cognitive_complexity is the SonarSource understandability metric
	// summed the same way; max_cognitive_complexity is the single worst method,
	// which is what the per-function thresholds (Sonar default 15, gocognit
	// default 30) are defined against.
	totalComplexity := 0
	totalCognitive := 0
	maxCognitive := 0
	for _, mth := range methods {
		totalComplexity += mth.CyclomaticComplexity
		totalCognitive += mth.CognitiveComplexity
		if mth.CognitiveComplexity > maxCognitive {
			maxCognitive = mth.CognitiveComplexity
		}
	}
	m["total_complexity"] = float64(totalComplexity)
	m["cognitive_complexity"] = float64(totalCognitive)
	m["max_cognitive_complexity"] = float64(maxCognitive)

	// --- OCP: type switches/assertions/reflection density ---
	var ts, ta, reflectN, stmts, ifaceParams int
	for _, mth := range methods {
		ts += mth.TypeSwitchCount
		ta += mth.TypeAssertCount
		reflectN += mth.ReflectUsageCount
		stmts += mth.StmtCount
		// Empty interfaces (any / interface{}) are excluded: they abandon type
		// information instead of abstracting behavior — typically the very
		// parameter a type switch downcasts — so rewarding them would offset the
		// penalty for the switch itself.
		for _, p := range mth.Params {
			if p.IsIface && !p.IsEmptyIface {
				ifaceParams++
			}
		}
	}
	m["type_switch_count"] = float64(ts)
	m["type_assert_count"] = float64(ta)
	m["reflect_count"] = float64(reflectN)
	m["total_stmts"] = float64(stmts)
	if stmts > 0 {
		m["type_check_density"] = float64(ts+ta+reflectN) / float64(stmts)
	}
	m["iface_param_count"] = float64(ifaceParams)

	// --- LSP: contract fidelity of implemented interface methods ---
	// A struct that embeds an in-package interface satisfies it through the
	// embedded value, so it is under contract even though it declares none of
	// the methods itself — without this the no-interface stop rule would mask
	// the embed-missing-override signal entirely.
	ifaceMethods := implementedInterfaceMethods(s, pkg)
	m["implements_interface"] = boolMetric(len(ifaceMethods) > 0 || embedsInPackageInterface(s, pkg))
	panicCount, noopCount := 0, 0
	for _, mth := range methods {
		if !ifaceMethods[mth.Name] {
			continue
		}
		if mth.HasUnconditionalPanic {
			panicCount++
		}
		if mth.IsNoop {
			noopCount++
		}
	}
	m["unconditional_panic_count"] = float64(panicCount)
	m["noop_count"] = float64(noopCount)
	m["embed_missing_override_count"] = float64(embedMissingOverrides(s, pkg))
	m["embedded_iface_injected"] = boolMetric(embeddedIfaceInjected(s, pkg))

	// --- ISP: public-surface size, large-interface implementation, cohesion ---
	m["is_decorator"] = boolMetric(isDecoratorPattern(s, pubMethods))
	if len(pubMethods) >= 4 {
		m["public_lcom4"] = float64(calculateLCOM4(pubMethods))
	}
	largePenalty, compositionBonus := ispInterfaceCoupling(s, pkg)
	m["isp_large_iface_penalty"] = largePenalty
	m["isp_composition_bonus"] = compositionBonus

	// --- DIP: weighted interface-vs-concrete dependency ratio ---
	dipMetrics(s, pkg, whitelist, m)

	return m
}

// InterfaceMetrics computes the metrics for an interface definition target.
func InterfaceMetrics(iface *model.InterfaceInfo) rules.Metrics {
	return rules.Metrics{
		"total_methods":  float64(iface.TotalMethods),
		"direct_methods": float64(len(iface.Methods)),
		"embed_count":    float64(len(iface.Embeds)),
	}
}

// embedMissingOverrides counts methods of embedded in-package interfaces that
// the struct does not override (an LSP smell: inheriting unintended behavior).
func embedMissingOverrides(s *model.StructInfo, pkg *model.PackageInfo) int {
	overridden := make(map[string]bool, len(s.Methods))
	for _, mth := range s.Methods {
		overridden[mth.Name] = true
	}
	count := 0
	for _, embed := range s.Embeddings {
		for _, iface := range pkg.Interfaces {
			if iface.Name != embed {
				continue
			}
			for _, im := range iface.Methods {
				if !overridden[im] {
					count++
				}
			}
		}
	}
	return count
}

// ispInterfaceCoupling sums the ISP large-interface penalty and composition
// bonus over every in-package interface the struct implements, mirroring the
// per-interface loop of the original ISP analyzer.
func ispInterfaceCoupling(s *model.StructInfo, pkg *model.PackageInfo) (penalty, bonus float64) {
	for _, iface := range pkg.Interfaces {
		implements := structImplements(s, iface)
		if iface.TotalMethods > 5 && implements {
			switch {
			case iface.TotalMethods <= 8:
				penalty += 10
			case iface.TotalMethods <= 12:
				penalty += 20
			default:
				penalty += 30
			}
		}
		if len(iface.Embeds) > 0 && implements {
			bonus += 10
		}
	}
	return penalty, bonus
}

// dipMetrics computes the weighted dependency totals used by DIP scoring and
// writes them into m. It reproduces the field/constructor/method-parameter
// weighting and the skipDep filtering of the original DIP analyzer.
func dipMetrics(s *model.StructInfo, pkg *model.PackageInfo, whitelist []string, m rules.Metrics) {
	var dw depWeight

	for _, f := range s.Fields {
		if skipDep(f.TypeName, f.IsIface, f.IsFunc, f.IsValue, s.Name, whitelist) {
			continue
		}
		dw.total += fieldDepWeight
		if f.IsIface {
			dw.iface += fieldDepWeight
		}
	}

	constructor := findConstructor(s.Name, pkg)
	if constructor != nil {
		for _, p := range constructor.Params {
			if skipDep(p.TypeName, p.IsIface, p.IsFunc, p.IsValue, s.Name, whitelist) {
				continue
			}
			dw.total += constructorDepWeight
			if p.IsIface {
				dw.iface += constructorDepWeight
			}
		}
	}

	structuralTotal := dw.total

	for _, mth := range s.Methods {
		if !mth.IsExported {
			continue
		}
		for _, p := range mth.Params {
			if skipDep(p.TypeName, p.IsIface, p.IsFunc, p.IsValue, s.Name, whitelist) {
				continue
			}
			dw.total += paramDepWeight
			if p.IsIface {
				dw.iface += paramDepWeight
			}
		}
	}

	m["weighted_dep_total"] = dw.total
	m["weighted_dep_iface"] = dw.iface
	m["structural_dep_total"] = structuralTotal
	if dw.total > 0 {
		m["iface_dep_ratio"] = dw.iface / dw.total
	}
	m["has_constructor_injection"] = boolMetric(constructor != nil && hasIfaceParams(constructor.Params))
	m["is_data_type"] = boolMetric(isDataType(s))
}

// boolMetric encodes a boolean fact as the 0/1 the rule engine compares against.
func boolMetric(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// metricNames is the complete vocabulary a rule may reference: the union of
// every key StructMetrics and InterfaceMetrics can produce. It is the source of
// truth used to reject typo'd metric names in user rules. A test verifies it
// covers every key the metric functions actually emit, so adding a metric
// without listing it here fails CI.
var metricNames = []string{
	// struct metrics
	"method_count", "public_method_count", "field_count", "has_fields",
	"lscc", "cohesion_method_count", "own_field_access_method_count", "total_complexity",
	"cognitive_complexity", "max_cognitive_complexity",
	"type_switch_count", "type_assert_count", "reflect_count", "total_stmts",
	"type_check_density", "iface_param_count",
	"implements_interface", "unconditional_panic_count", "noop_count",
	"embed_missing_override_count", "embedded_iface_injected",
	"is_decorator", "public_lcom4", "isp_large_iface_penalty", "isp_composition_bonus",
	"weighted_dep_total", "weighted_dep_iface", "structural_dep_total",
	"iface_dep_ratio", "has_constructor_injection", "is_data_type",
	// interface metrics
	"total_methods", "direct_methods", "embed_count",
}

// MetricNames returns the names of every metric a rule may reference. Passing
// it to rule-engine construction lets the engine reject rules that reference an
// unknown (typically misspelled) metric.
func MetricNames() []string {
	out := make([]string, len(metricNames))
	copy(out, metricNames)
	return out
}
