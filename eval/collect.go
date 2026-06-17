package eval

import (
	"fmt"

	"github.com/harakeishi/go-solid-score/analyzer"
	"github.com/harakeishi/go-solid-score/model"
	"github.com/harakeishi/go-solid-score/scorer"
)

// CollectDocLabels extracts the inline `// solid:want` ground-truth labels from
// every struct and interface in the parsed packages, assigning each label the
// target ID `<pkgPath>.<name>`. That ID generation MUST match
// scorer.targetID's primary branch so labels join to scored results; the
// fallback `<file>:<name>` form is not reproduced here because every parsed
// package in practice carries a package path, and a fallback ID would not be
// stable enough to anchor a ground-truth label to.
//
// Interfaces are included even though the scorer currently scores only structs:
// their labels simply find no matching score and are skipped at join time
// (eval.classifyUnits), which is the correct behaviour until interface scoring
// lands.
func CollectDocLabels(pkgs []*model.PackageInfo) ([]Label, error) {
	var labels []Label
	for _, pkg := range pkgs {
		for _, s := range pkg.Structs {
			ls, err := docLabelsFor(pkg.PkgPath, s.Name, s.Doc)
			if err != nil {
				return nil, err
			}
			labels = append(labels, ls...)
		}
		for _, iface := range pkg.Interfaces {
			ls, err := docLabelsFor(pkg.PkgPath, iface.Name, iface.Doc)
			if err != nil {
				return nil, err
			}
			labels = append(labels, ls...)
		}
	}
	return labels, nil
}

// docLabelsFor parses a single type's doc comment and stamps each resulting
// label with the joined target ID.
func docLabelsFor(pkgPath, name, doc string) ([]Label, error) {
	parsed, err := ParseDocLabels(doc)
	if err != nil {
		return nil, fmt.Errorf("%s.%s: %w", pkgPath, name, err)
	}
	id := pkgPath + "." + name
	for i := range parsed {
		parsed[i].ID = id
	}
	return parsed, nil
}

// CollectScores projects scored results into the nested map BuildReport joins
// against: target ID -> principle -> score.
func CollectScores(results []*scorer.ScoreResult) map[string]map[analyzer.Principle]float64 {
	scored := make(map[string]map[analyzer.Principle]float64, len(results))
	for _, r := range results {
		scored[r.TargetID()] = r.Scores
	}
	return scored
}
