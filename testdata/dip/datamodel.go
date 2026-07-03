package dip

import "strings"

// Param is a behavior-less data record aggregated by Doc below, mirroring the
// AST-model shape (model.FuncInfo holding []*ParamInfo) that made every data
// model score DIP=0.
type Param struct {
	Name string
	Kind int
}

// Doc mirrors an AST/data-model aggregate: it stores pointer collections of
// other in-package data records and exposes only accessors and convention
// methods. Holding your own records — even by pointer — is data composition,
// not an uninverted service dependency, so DIP must report "not applicable"
// instead of a confident zero. Contrast with CollaboratorOwner/Pipeline, whose
// behavioral methods drive their pointer collections and stay violations.
// solid:want DIP=na reason="behavior-less data record aggregate (accessor and convention methods only); its []*Param field is stored data, not a collaborator — DIP not applicable"
type Doc struct {
	Title  string
	Params []*Param
}

// String is a fmt.Stringer convention method — formatting own data.
func (d *Doc) String() string { return d.Title }

// Exported is a pure accessor: it reshapes own data and calls nothing.
func (d *Doc) Exported() []*Param {
	var out []*Param
	for _, p := range d.Params {
		if p.Kind > 0 {
			out = append(out, p)
		}
	}
	return out
}

// Options is a nested, method-less config block held by value in Tool below.
type Options struct {
	Retries int
	Tag     string
}

// Tool owns behavior (Describe delegates to strings.ToUpper, so it is not a
// data type), but its only struct field is a method-less value config block. A bare value struct with no methods exposes nothing to call
// into, so it is stored data — with it skipped, Tool owns no dependencies and
// DIP is not applicable, rather than a confident zero for holding its own
// configuration.
// solid:want DIP=na reason="only owns a method-less value config block (stored data, not a collaborator); no dependencies remain — DIP not applicable"
type Tool struct {
	Name string
	Opts Options
}

// Describe delegates to strings.ToUpper while producing its result, keeping
// Tool behavioral (not a data type) so the value-field skip is what this case
// exercises.
func (t *Tool) Describe() string { return strings.ToUpper(t.Name) }
