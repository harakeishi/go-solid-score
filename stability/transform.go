// Package stability measures the noise floor of the SOLID scorer: the amount a
// target's score moves under edits that change a program's text but not its
// meaning. The scorer is a deterministic static analysis, so its measurement
// noise floor should be exactly zero — every semantics-preserving edit must
// leave every score bit-for-bit (well, 0.1-for-0.1) unchanged. Any movement is
// a precision bug, of exactly the kind the project has hit before (e.g. the
// LCOM4 penalty that swung on whether a method touched a receiver field).
//
// The approach is metamorphic testing, the standard technique for validating
// static analyzers against semantics-preserving program transformations (as in
// Statfier, evaluated on PMD/CheckStyle): apply a transformation whose
// metamorphic relation is "the verdict must not change", then assert it didn't.
// The transformations here mirror the canonical catalog — identifier renaming,
// declaration reordering, and comment/whitespace injection.
package stability

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

// Transform is one semantics-preserving source rewrite. Apply takes the source
// of a single Go file and returns an equivalent file that differs in text but
// not in meaning. It must keep the file compilable and must not change which
// types exist or what they are named (renaming a type would change its scoring
// identity, which is a different concern from measuring per-target noise).
type Transform struct {
	Name  string
	Apply func(src []byte) ([]byte, error)
}

// Transforms returns the catalog of semantics-preserving transformations the
// noise-floor harness sweeps over. Each targets a distinct way the scorer could
// accidentally depend on non-semantic surface detail:
//
//   - reorder-decls exercises order-independence: the score must not depend on
//     the order types and methods appear in a file.
//   - rename-receivers exercises identifier-independence, and specifically the
//     field-access detection that LCOM4/SRP rely on: renaming a method receiver
//     must not change which fields a method is seen to touch.
//   - inject-comments exercises comment/whitespace-independence: the scorer must
//     ignore comments entirely, even though the parser deliberately keeps them
//     (for the evaluate harness's inline labels).
func Transforms() []Transform {
	return []Transform{
		{Name: "reorder-decls", Apply: reorderDecls},
		{Name: "rename-receivers", Apply: renameReceivers},
		{Name: "inject-comments", Apply: injectComments},
	}
}

// reorderDecls reverses the order of top-level declarations, keeping import
// declarations first (Go requires imports before other declarations). Comments
// are dropped rather than preserved: reattaching free-floating comments to
// reordered nodes is what corrupts go/format output, and dropping them is itself
// semantics-preserving.
func reorderDecls(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "src.go", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	var imports, others []ast.Decl
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			imports = append(imports, d)
			continue
		}
		others = append(others, d)
	}
	for i, j := 0, len(others)-1; i < j; i, j = i+1, j-1 {
		others[i], others[j] = others[j], others[i]
	}
	f.Decls = append(imports, others...)

	return formatFile(fset, f)
}

// renameReceivers renames every method receiver to a fixed fresh identifier and
// rewrites its uses in the method body. Receivers are method-local, so this is
// purely cosmetic — but it drives the receiver-field-access path that cohesion
// scoring keys on, so a scorer that (say) matched field accesses by the textual
// receiver name would betray itself here.
func renameReceivers(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "src.go", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	const newName = "rcv"
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		field := fn.Recv.List[0]
		if len(field.Names) != 1 {
			continue // an unnamed receiver — nothing accesses it, nothing to rename
		}
		old := field.Names[0].Name
		if old == "_" || old == newName {
			continue
		}
		field.Names[0].Name = newName
		if fn.Body == nil {
			continue
		}
		astutil.Apply(fn.Body, func(c *astutil.Cursor) bool {
			id, ok := c.Node().(*ast.Ident)
			if !ok || id.Name != old {
				return true
			}
			// Skip identifiers that are a selector's field/method name or a
			// composite-literal key — those are not the receiver, they only
			// happen to share its spelling.
			switch c.Name() {
			case "Sel", "Key":
				return true
			}
			id.Name = newName
			return true
		}, nil)
	}

	return formatFile(fset, f)
}

// injectComments inserts a full-line comment before every top-level func/type
// declaration and appends a trailing comment. Working on lines (rather than the
// AST) keeps the inserted comments from being scrambled by the printer, and
// inserting whole comment lines between declarations can never break
// compilation. It verifies the result still parses before returning.
func injectComments(src []byte) ([]byte, error) {
	lines := strings.Split(string(src), "\n")
	out := make([]string, 0, len(lines)+len(lines)/4+1)
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "type ") {
			out = append(out, "// stability: injected noise comment")
		}
		out = append(out, line)
	}
	out = append(out, "// stability: trailing noise comment")
	result := []byte(strings.Join(out, "\n"))

	// Guard against a fixture whose surface defeats the line heuristic (e.g. a
	// "func "/"type " at the start of a raw-string line): if the rewrite no
	// longer parses, that is a harness bug, not a scorer finding.
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "src.go", result, parser.SkipObjectResolution); err != nil {
		return nil, err
	}
	return result, nil
}

func formatFile(fset *token.FileSet, f *ast.File) ([]byte, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
