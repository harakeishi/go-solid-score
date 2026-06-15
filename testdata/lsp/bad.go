package lsp

// Saver interface for saving data.
type Saver interface {
	Save(data []byte) error
	Load(id string) ([]byte, error)
}

// ReadOnlySaver violates LSP by panicking on Save.
type ReadOnlySaver struct{}

func (r *ReadOnlySaver) Save(data []byte) error {
	panic("read-only: cannot save")
}

func (r *ReadOnlySaver) Load(id string) ([]byte, error) {
	return nil, nil
}

// GuardedSaver implements Saver and panics ONLY on invalid input (a fail-fast
// guard). This is idiomatic Go, not an LSP violation, so it must not be
// penalized for the panic.
type GuardedSaver struct {
	data map[string][]byte
}

func (g *GuardedSaver) Save(data []byte) error {
	if data == nil {
		panic("GuardedSaver: nil data")
	}
	g.data["k"] = data
	return nil
}

func (g *GuardedSaver) Load(id string) ([]byte, error) {
	if id == "" {
		panic("GuardedSaver: empty id")
	}
	return g.data[id], nil
}

// NoopSaver violates LSP with no-op implementations.
type NoopSaver struct{}

func (n *NoopSaver) Save(data []byte) error {
	return nil
}

func (n *NoopSaver) Load(id string) ([]byte, error) {
	return nil, nil
}
