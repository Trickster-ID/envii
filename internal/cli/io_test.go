package cli_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/trickylab/envii/internal/cli"
)

type testIO struct {
	env    map[string]string
	stdin  *bytes.Buffer
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	pass   []byte
}

func (t *testIO) Getenv(k string) string {
	if t.env == nil {
		return ""
	}
	return t.env[k]
}
func (t *testIO) Stdin() io.Reader                     { return t.stdin }
func (t *testIO) Stdout() io.Writer                    { return t.stdout }
func (t *testIO) Stderr() io.Writer                    { return t.stderr }
func (t *testIO) ReadPassword(fd int) ([]byte, error)  { return t.pass, nil }
func (t *testIO) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

func TestIOAbstraction(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		key  string
		want string
	}{
		{name: "empty", env: nil, key: "ENVII_PASSPHRASE", want: ""},
		{name: "set", env: map[string]string{"ENVII_PASSPHRASE": "secret"}, key: "ENVII_PASSPHRASE", want: "secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tio := &testIO{
				env:    tt.env,
				stdin:  &bytes.Buffer{},
				stdout: &bytes.Buffer{},
				stderr: &bytes.Buffer{},
				pass:   []byte("pass"),
			}
			cli.SetIO(tio)
			t.Cleanup(func() { cli.SetIO(nil) })

			if got := cli.CurrentIO().Getenv(tt.key); got != tt.want {
				t.Fatalf("Getenv got %q want %q", got, tt.want)
			}
			if _, err := cli.CurrentIO().ReadPassword(0); err != nil {
				t.Fatalf("ReadPassword: %v", err)
			}
		})
	}
}
