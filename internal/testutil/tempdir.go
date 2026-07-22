package testutil

import "testing"

// TempDir returns a temporary directory that is cleaned up after the test.
func TempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}
