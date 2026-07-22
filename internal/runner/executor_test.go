package runner_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/trickylab/envii/internal/model"
	"github.com/trickylab/envii/internal/runner"
)

type fakeExecutor struct {
	called bool
	argv   []string
	env    []string
}

func (f *fakeExecutor) Run(argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error {
	f.called = true
	f.argv = append([]string(nil), argv...)
	f.env = append([]string(nil), env...)
	_, _ = stdout.Write([]byte("ok"))
	return nil
}

func TestRunWithInjectedExecutor(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		key  string
		val  string
	}{
		{name: "echo", argv: []string{"echo"}, key: "K", val: "V"},
		{name: "true", argv: []string{"true"}, key: "A", val: "B"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := &fakeExecutor{}
			r := runner.New(runner.WithExecutor(fe))
			code, err := r.Run(&model.Env{Vars: []*model.Var{{Key: tt.key, Value: tt.val}}}, tt.argv)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if code != 0 {
				t.Fatalf("code %d, want 0", code)
			}
			if !fe.called {
				t.Fatal("expected executor called")
			}
			if len(fe.argv) == 0 || fe.argv[0] != tt.argv[0] {
				t.Fatalf("argv got %v, want start %v", fe.argv, tt.argv)
			}
			found := false
			want := tt.key + "=" + tt.val
			for _, e := range fe.env {
				if e == want {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("env missing %q in %v", want, fe.env)
			}
			_ = bytes.Buffer{} // silence unused if needed
		})
	}
}
