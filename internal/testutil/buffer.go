package testutil

import "bytes"

// BufferIO holds in-memory stdio buffers for testing.
type BufferIO struct {
	Stdin  *bytes.Buffer
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}

// NewBufferIO returns a new BufferIO with initialized buffers.
func NewBufferIO() *BufferIO {
	return &BufferIO{
		Stdin:  &bytes.Buffer{},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
}
