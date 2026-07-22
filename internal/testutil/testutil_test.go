package testutil_test

import (
	"testing"

	"github.com/trickylab/envii/internal/testutil"
)

func TestTempDir(t *testing.T) {
	dir := testutil.TempDir(t)
	if dir == "" {
		t.Fatal("empty temp dir")
	}
}

func TestNewBufferIO(t *testing.T) {
	io := testutil.NewBufferIO()
	if io.Stdin == nil || io.Stdout == nil || io.Stderr == nil {
		t.Fatal("buffers nil")
	}
	if _, err := io.Stdout.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if io.Stdout.String() != "x" {
		t.Fatalf("got %q", io.Stdout.String())
	}
}
