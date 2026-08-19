package cli

import (
	"io"
	"os"

	"golang.org/x/term"
)

// IO abstracts CLI environment and stdio for testability.
type IO interface {
	Getenv(key string) string
	Stdin() io.Reader
	Stdout() io.Writer
	Stderr() io.Writer
	ReadPassword(fd int) ([]byte, error)
	ReadFile(path string) ([]byte, error)
}

// OSIO implements IO using the real OS.
type OSIO struct{}

func (OSIO) Getenv(key string) string             { return os.Getenv(key) }
func (OSIO) Stdin() io.Reader                     { return os.Stdin }
func (OSIO) Stdout() io.Writer                    { return os.Stdout }
func (OSIO) Stderr() io.Writer                    { return os.Stderr }
func (OSIO) ReadPassword(fd int) ([]byte, error)  { return term.ReadPassword(fd) }
func (OSIO) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

// defaultIO is used when no IO is injected.
var defaultIO IO = OSIO{}

// SetIO replaces the package-level IO (tests). Pass nil to restore OSIO.
func SetIO(io IO) {
	if io == nil {
		defaultIO = OSIO{}
		return
	}
	defaultIO = io
}

// CurrentIO returns the active IO implementation.
func CurrentIO() IO { return defaultIO }
