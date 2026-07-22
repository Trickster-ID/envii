package store

import "os"

// FS abstracts filesystem operations for store testability.
type FS interface {
	UserConfigDir() (string, error)
	Stat(name string) (os.FileInfo, error)
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Rename(oldpath, newpath string) error
}

// OSFS implements FS using the real OS.
type OSFS struct{}

func (OSFS) UserConfigDir() (string, error) { return os.UserConfigDir() }
func (OSFS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}
func (OSFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }
func (OSFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}
func (OSFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
func (OSFS) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// Option configures a Store.
type Option func(*Store)

// WithFS injects a filesystem implementation.
func WithFS(fs FS) Option {
	return func(s *Store) { s.fs = fs }
}

// WithPath overrides the vault path.
func WithPath(path string) Option {
	return func(s *Store) { s.Path = path }
}
