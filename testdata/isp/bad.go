package isp

// FatInterface forces implementors to depend on methods they don't use.
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

// FatImpl has a bloated public interface.
type FatImpl struct {
	data   []byte
	locked bool
	name   string
}

func (f *FatImpl) Read(p []byte) (int, error)                { return copy(p, f.data), nil }
func (f *FatImpl) Write(p []byte) (int, error)               { f.data = append(f.data, p...); return len(p), nil }
func (f *FatImpl) Close() error                              { f.data = nil; return nil }
func (f *FatImpl) Seek(offset int64, whence int) (int64, error) { return 0, nil }
func (f *FatImpl) Stat() (string, error)                     { return f.name, nil }
func (f *FatImpl) Sync() error                               { return nil }
func (f *FatImpl) Truncate(size int64) error                 { f.data = f.data[:size]; return nil }
func (f *FatImpl) ReadAt(p []byte, off int64) (int, error)   { return copy(p, f.data[off:]), nil }
func (f *FatImpl) WriteAt(p []byte, off int64) (int, error)  { copy(f.data[off:], p); return len(p), nil }
func (f *FatImpl) Lock() error                               { f.locked = true; return nil }
func (f *FatImpl) Unlock() error                             { f.locked = false; return nil }
