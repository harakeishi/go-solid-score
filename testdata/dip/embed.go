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
