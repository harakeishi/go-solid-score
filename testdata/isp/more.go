package isp

// Backend mixes storage, administration and metrics into one 12-method
// contract; a metrics-only client is still coupled to compaction and user
// management.
// solid:want ISP=violation reason="12 methods across three separable client roles (storage, admin, metrics) — clients depend on methods they never call"
type Backend interface {
	Get(key string) ([]byte, error)
	Put(key string, val []byte) error
	Delete(key string) error
	Scan(prefix string) ([]string, error)
	Compact() error
	Backup(path string) error
	Restore(path string) error
	AddUser(name string) error
	RemoveUser(name string) error
	Stats() map[string]int64
	ResetStats()
	Ping() error
}

// MetricsClient is forced to implement all of Backend to be plugged in as one,
// stubbing the nine methods it has no use for — the implementor-side cost of
// the fat interface.
// solid:want ISP=violation reason="forced implementation of the fat Backend interface: only the metrics methods are real, the rest are stubs it must carry anyway"
// solid:want LSP=violation reason="Liskov: nine contract methods are silent no-op stubs claiming success — cannot substitute for Backend"
type MetricsClient struct {
	stats map[string]int64
}

func (m *MetricsClient) Get(key string) ([]byte, error)       { return nil, nil }
func (m *MetricsClient) Put(key string, val []byte) error     { return nil }
func (m *MetricsClient) Delete(key string) error              { return nil }
func (m *MetricsClient) Scan(prefix string) ([]string, error) { return nil, nil }
func (m *MetricsClient) Compact() error                       { return nil }
func (m *MetricsClient) Backup(path string) error             { return nil }
func (m *MetricsClient) Restore(path string) error            { return nil }
func (m *MetricsClient) AddUser(name string) error            { return nil }
func (m *MetricsClient) RemoveUser(name string) error         { return nil }
func (m *MetricsClient) Stats() map[string]int64              { return m.stats }
func (m *MetricsClient) ResetStats()                          { m.stats = map[string]int64{} }
func (m *MetricsClient) Ping() error                          { return nil }

// MegaFacade exposes seventeen public methods spanning parsing, rendering,
// caching and IO — a public surface no single client consumes.
// solid:want ISP=violation reason="17 public methods across four unrelated concerns; any client sees a severely bloated surface"
type MegaFacade struct {
	src   string
	html  string
	cache map[string]string
	path  string
}

func (f *MegaFacade) ParseSource() error       { return nil }
func (f *MegaFacade) ParseFragment() error     { return nil }
func (f *MegaFacade) Tokens() []string         { return nil }
func (f *MegaFacade) SyntaxOK() bool           { return f.src != "" }
func (f *MegaFacade) RenderHTML() string       { return f.html }
func (f *MegaFacade) RenderText() string       { return f.src }
func (f *MegaFacade) RenderJSON() string       { return "{}" }
func (f *MegaFacade) Theme() string            { return "default" }
func (f *MegaFacade) SetTheme(t string)        { _ = t }
func (f *MegaFacade) CacheGet(k string) string { return f.cache[k] }
func (f *MegaFacade) CachePut(k, v string)     { f.cache[k] = v }
func (f *MegaFacade) CacheClear()              { f.cache = nil }
func (f *MegaFacade) CacheLen() int            { return len(f.cache) }
func (f *MegaFacade) OpenFile() error          { return nil }
func (f *MegaFacade) SaveFile() error          { return nil }
func (f *MegaFacade) FilePath() string         { return f.path }
func (f *MegaFacade) SetFilePath(p string)     { f.path = p }

// Putter is a single-method role interface a write-only client depends on.
// solid:want ISP=ok reason="single-method role interface; clients depend on exactly what they use"
type Putter interface {
	Put(key string, val []byte) error
}

// Getter is the read-side role interface.
// solid:want ISP=ok reason="single-method role interface; clients depend on exactly what they use" split=train
type Getter interface {
	Get(key string) ([]byte, error)
}

// KV composes the two roles for the minority of clients that need both.
// solid:want ISP=ok reason="composed from Putter/Getter role interfaces via embedding — the io.ReadWriter pattern"
type KV interface {
	Putter
	Getter
}

// Cache has a compact, cohesive public surface: four methods, one concern.
// solid:want ISP=ok reason="four public methods around one concern (cache access); no client is over-coupled"
type Cache struct {
	entries map[string][]byte
}

func (c *Cache) Get(key string) []byte    { return c.entries[key] }
func (c *Cache) Set(key string, v []byte) { c.entries[key] = v }
func (c *Cache) Invalidate(key string)    { delete(c.entries, key) }
func (c *Cache) Size() int                { return len(c.entries) }
