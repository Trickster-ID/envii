package runner

import (
	"io"
	"os/exec"
)

// Executor abstracts subprocess execution.
type Executor interface {
	Run(argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error
}

// OSExecutor runs commands via os/exec.
type OSExecutor struct{}

// Run executes argv with the given env and stdio.
func (OSExecutor) Run(argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = env
	return cmd.Run()
}

// Runner executes commands with injected environment variables.
type Runner struct {
	exec Executor
}

// Option configures a Runner.
type Option func(*Runner)

// WithExecutor injects a subprocess executor.
func WithExecutor(e Executor) Option {
	return func(r *Runner) { r.exec = e }
}

// New returns a Runner. Default executor is OSExecutor.
func New(opts ...Option) *Runner {
	r := &Runner{exec: OSExecutor{}}
	for _, opt := range opts {
		opt(r)
	}
	if r.exec == nil {
		r.exec = OSExecutor{}
	}
	return r
}
