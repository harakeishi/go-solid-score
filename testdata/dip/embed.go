package dip

// Engine is a concrete type used as an embedded dependency.
type Engine struct{ rpm int }

func (e *Engine) Start() {}

// ConcreteEmbedder embeds a concrete type. Embedding is the tightest form of
// structural coupling, so it must count as a concrete DIP dependency — not be
// ignored (which would vacuously award a perfect score for the densest possible
// coupling).
// solid:want DIP=violation reason="embeds a concrete type (*Engine); embedding is a concrete structural dependency, not exempt from DIP"
type ConcreteEmbedder struct {
	*Engine
}

func (c *ConcreteEmbedder) Drive() {}

// IfaceEmbedder embeds an interface (the io.ReadWriteCloser pattern). Embedding
// an abstraction is good DIP and must earn abstraction credit.
// solid:want DIP=ok reason="embeds an interface (Logger); depending on an abstraction satisfies DIP"
type IfaceEmbedder struct {
	Logger
}

func (i *IfaceEmbedder) Run() {}

// Mixed owns BOTH a methodful concrete collaborator (*Engine, which has Start())
// AND a methodless data struct (*stage, a DTO). Because at least one dependency
// is an unambiguous behavioral collaborator, the methodless-data-struct
// confidence cap must NOT fire: the coupling is real, not ambiguous. This guards
// the `structuralData == structuralConcrete` boundary (a `>= 1` mutation would
// wrongly lower confidence here). The constructor injects the data struct,
// covering the constructor-param IsData path as well.
// solid:want DIP=violation reason="owns a concrete collaborator (*Engine) and a data struct (*stage); a genuine concrete dependency, and not an all-data-struct case"
type Mixed struct {
	engine *Engine
	st     *stage
}

func NewMixed(st *stage) *Mixed {
	return &Mixed{engine: &Engine{}, st: st}
}

func (m *Mixed) Work() {}

// CtorData has no concrete struct fields; its only structural concrete
// dependency is a methodless data struct (*stage) injected through the
// constructor. This exercises the constructor-param IsData path: if the
// constructor's `structuralData++` were dropped, the cap would stop firing and
// confidence would (wrongly) stay high. With it, every structural concrete dep
// is a data struct, so the cap fires and confidence is lowered.
// solid:want DIP=violation reason="owns a data struct (*stage) via constructor injection; concrete dependency, ambiguous (low confidence)"
type CtorData struct {
	count int // value field, not a concrete dependency
}

func NewCtorData(st *stage) *CtorData {
	_ = st
	return &CtorData{}
}
