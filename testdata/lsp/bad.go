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

// NoopSaver violates LSP with no-op implementations.
type NoopSaver struct{}

func (n *NoopSaver) Save(data []byte) error {
	return nil
}

func (n *NoopSaver) Load(id string) ([]byte, error) {
	return nil, nil
}
