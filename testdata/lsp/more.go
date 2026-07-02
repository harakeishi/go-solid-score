package lsp

// Store is a three-method contract used by the partial implementations below.
type Store interface {
	Put(key string, val []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
}

// PartialStore stubs out most of the contract with panics: only Get works.
// Distinct from ReadOnlySaver, which panics in a single method — this is the
// "implements the interface in name only" variant.
// solid:want LSP=violation reason="Liskov: Put and Delete both panic unconditionally — the type satisfies Store syntactically but cannot substitute for it"
type PartialStore struct {
	data map[string][]byte
}

func (p *PartialStore) Put(key string, val []byte) error {
	panic("PartialStore is read-only")
}

func (p *PartialStore) Get(key string) ([]byte, error) {
	return p.data[key], nil
}

func (p *PartialStore) Delete(key string) error {
	panic("PartialStore is read-only")
}

// ZeroStore silently returns zero values for the whole contract: every call
// claims success while doing nothing — the postcondition-weakening flavour of
// the same defect PartialStore expresses with panics.
// solid:want LSP=violation reason="Liskov: all three contract methods are silent no-ops returning zero values, weakening every postcondition of Store"
type ZeroStore struct{}

func (z *ZeroStore) Put(key string, val []byte) error { return nil }

func (z *ZeroStore) Get(key string) ([]byte, error) { return nil, nil }

func (z *ZeroStore) Delete(key string) error { return nil }

// Codec is a wide in-package contract used to exercise the embedding case.
type Codec interface {
	Encode(v any) ([]byte, error)
	Decode(b []byte, v any) error
	EncodeStream(vs []any) ([]byte, error)
	DecodeStream(b []byte) ([]any, error)
	ContentType() string
	Reset()
	Clone() Codec
}

// LazyCodec embeds the Codec interface to satisfy it but overrides only one
// of seven methods; calling any inherited method on a zero LazyCodec nil-panics
// at runtime. Interface embedding used as fake inheritance is the Go shape of
// "subclass inherits behaviour it does not support".
// solid:want LSP=violation reason="Liskov: embeds Codec but overrides 1 of 7 methods — the remaining methods dispatch to a nil embedded interface and panic, so it cannot substitute for Codec"
type LazyCodec struct {
	Codec
}

func (l *LazyCodec) ContentType() string { return "application/x-lazy" }

// LoggingCodec is the legitimate twin of LazyCodec: the identical AST shape
// (embed Codec, override one method), but the constructor injects a real
// implementation for the embedded interface to delegate to. This is the
// standard Go decorator/middleware pattern and must NOT be penalized for the
// methods it deliberately does not override.
// solid:want LSP=ok reason="decorator: embedded Codec is constructor-injected, so non-overridden methods delegate to a real implementation — fully substitutable"
type LoggingCodec struct {
	Codec
	logs []string
}

func NewLoggingCodec(inner Codec) *LoggingCodec {
	return &LoggingCodec{Codec: inner}
}

func (l *LoggingCodec) Encode(v any) ([]byte, error) {
	l.logs = append(l.logs, "encode")
	return l.Codec.Encode(v)
}

// MemStore fully honours the Store contract.
// solid:want LSP=ok reason="all contract methods are real implementations with honest results; fully substitutable for Store"
type MemStore struct {
	data map[string][]byte
}

func (m *MemStore) Put(key string, val []byte) error {
	if m.data == nil {
		m.data = map[string][]byte{}
	}
	m.data[key] = val
	return nil
}

func (m *MemStore) Get(key string) ([]byte, error) {
	return m.data[key], nil
}

func (m *MemStore) Delete(key string) error {
	delete(m.data, key)
	return nil
}

// Flusher is a two-method contract where one method may legitimately have
// nothing to do.
type Flusher interface {
	Write(p []byte) (int, error)
	Flush() error
}

// BufferlessWriter writes straight through, so its Flush is genuinely a no-op.
// A single no-op in an otherwise real implementation is idiomatic (e.g. Close
// on a type with nothing to release) and must stay above the violation
// threshold — the boundary case for no-op detection.
// solid:want LSP=ok reason="Write is a real implementation; Flush is legitimately empty because nothing is buffered — one idiomatic no-op is not a substitutability defect" split=train
type BufferlessWriter struct {
	out []byte
}

func (b *BufferlessWriter) Write(p []byte) (int, error) {
	b.out = append(b.out, p...)
	return len(p), nil
}

func (b *BufferlessWriter) Flush() error { return nil }
