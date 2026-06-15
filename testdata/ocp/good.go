package ocp

// Processor uses an interface for extensibility (no type switches needed).
type Processor struct {
	handler Handler
}

// Handler is an interface allowing extension without modifying Processor.
type Handler interface {
	Handle(data []byte) error
}

func NewProcessor(h Handler) *Processor {
	return &Processor{handler: h}
}

func (p *Processor) Process(data []byte) error {
	return p.handler.Handle(data)
}

// Flushable is an optional capability interface.
type Flushable interface {
	Flush() error
}

// FeatureDispatcher detects an optional capability via a comma-ok assertion to
// an *interface*. This is open for extension — any new type implementing
// Flushable works without changing this code — so OCP must not penalize it,
// unlike a downcast to a concrete type.
type FeatureDispatcher struct {
	sink any
}

func (fd *FeatureDispatcher) Dispatch() error {
	if f, ok := fd.sink.(Flushable); ok {
		return f.Flush()
	}
	return nil
}
