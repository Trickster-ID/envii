package runner_test

import (
	"errors"
	"io"
	"testing"

	"github.com/trickylab/envii/internal/model"
	"github.com/trickylab/envii/internal/runner"
)

type errExecutor struct{ err error }

func (e errExecutor) Run(argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return e.err
}

func TestNewNilExecutorFallsBack(t *testing.T) {
	r := runner.New(runner.WithExecutor(nil))
	// package Run convenience still works via real OS for empty argv error
	code, err := r.Run(&model.Env{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if code != 1 {
		t.Fatalf("code %d", code)
	}
}

func TestRunNonExitError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "generic", err: errors.New("exec not found")},
		{name: "wrapped", err: errors.New("no such file")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runner.New(runner.WithExecutor(errExecutor{err: tt.err}))
			code, err := r.Run(&model.Env{Vars: []*model.Var{{Key: "A", Value: "1"}}}, []string{"false"})
			if err == nil {
				t.Fatal("expected error")
			}
			if code != 1 {
				t.Fatalf("code %d want 1", code)
			}
		})
	}
}

func TestPackageRunDelegates(t *testing.T) {
	_, err := runner.Run(&model.Env{}, nil)
	if err == nil {
		t.Fatal("expected error for empty argv")
	}
}
