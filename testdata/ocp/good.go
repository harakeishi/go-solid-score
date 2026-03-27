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
