package isp

// FatInterface forces implementors to depend on methods they don't use.
// solid:want ISP=violation reason="Martin ISP / Pike 'the bigger the interface, the weaker the abstraction': 11 methods force clients to depend on methods they don't use — the interface-definition recall guard"
type FatInterface interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Close() error
	Seek(offset int64, whence int) (int64, error)
	Stat() (string, error)
	Sync() error
	Truncate(size int64) error
	ReadAt(p []byte, off int64) (int, error)
	WriteAt(p []byte, off int64) (int, error)
	Lock() error
	Unlock() error
}

// Closer is a small role interface, embedded by FatEmbedInterface below.
// solid:want ISP=ok reason="single-method role interface (io.Closer idiom); minimal client coupling"
type Closer interface {
	Close() error
}

// FatEmbedInterface embeds a small role interface (Closer) but still declares
// 10 methods directly. It is structurally a fat interface — the single embed
// must NOT rescue it from the ISP violation threshold.
// solid:want ISP=violation reason="11 effective methods (10 declared directly + 1 embedded); embedding a single role interface does not make a directly-bloated interface ISP-faithful — guards against the embed-bonus false negative"
type FatEmbedInterface interface {
	Closer
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Seek(offset int64, whence int) (int64, error)
	Stat() (string, error)
	Sync() error
	Truncate(size int64) error
	ReadAt(p []byte, off int64) (int, error)
	WriteAt(p []byte, off int64) (int, error)
	Lock() error
	Unlock() error
}

// FatImpl has a bloated public interface.
// solid:want ISP=violation reason="Lanza&Marinescu/Martin fat interface: 11 public methods forcing clients to depend on methods they don't use — the recall guard for ISP relaxation"
type FatImpl struct {
	data   []byte
	locked bool
	name   string
}

func (f *FatImpl) Read(p []byte) (int, error)                   { return copy(p, f.data), nil }
func (f *FatImpl) Write(p []byte) (int, error)                  { f.data = append(f.data, p...); return len(p), nil }
func (f *FatImpl) Close() error                                 { f.data = nil; return nil }
func (f *FatImpl) Seek(offset int64, whence int) (int64, error) { return 0, nil }
func (f *FatImpl) Stat() (string, error)                        { return f.name, nil }
func (f *FatImpl) Sync() error                                  { return nil }
func (f *FatImpl) Truncate(size int64) error                    { f.data = f.data[:size]; return nil }
func (f *FatImpl) ReadAt(p []byte, off int64) (int, error)      { return copy(p, f.data[off:]), nil }
func (f *FatImpl) WriteAt(p []byte, off int64) (int, error) {
	copy(f.data[off:], p)
	return len(p), nil
}
func (f *FatImpl) Lock() error   { f.locked = true; return nil }
func (f *FatImpl) Unlock() error { f.locked = false; return nil }
