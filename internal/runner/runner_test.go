package runner

import (
	"os/exec"
	"runtime"
	"testing"

	"github.com/trickylab/envii/internal/model"
)

func TestDotenv(t *testing.T) {
	env := &model.Env{Vars: []*model.Var{
		{Key: "ZED", Value: "last"},
		{Key: "ABC", Value: "first"},
		{Key: "QUOTED", Value: "has space"},
	}}

	got := Dotenv(env)
	want := "ABC=first\nQUOTED=\"has space\"\nZED=last\n"
	if got != want {
		t.Fatalf("Dotenv mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestRunNoCommand(t *testing.T) {
	if _, err := Run(&model.Env{}, nil); err == nil {
		t.Fatal("expected error for empty argv")
	}
}

func TestRunSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command differs on windows")
	}
	code, err := Run(&model.Env{Vars: []*model.Var{{Key: "ENVII_TEST_RUN", Value: "ok"}}}, []string{"sh", "-c", "test \"$ENVII_TEST_RUN\" = ok"})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("got exit code %d, want 0", code)
	}
}

func TestRunExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell command differs on windows")
	}
	code, err := Run(&model.Env{}, []string{"sh", "-c", "exit 7"})
	if err != nil {
		t.Fatal(err)
	}
	if code != 7 {
		t.Fatalf("got exit code %d, want 7", code)
	}
}

func TestAsExitError(t *testing.T) {
	var target *exec.ExitError
	if asExitError(nil, &target) {
		t.Fatal("nil error should not match")
	}
	if asExitError(assertErr{}, &target) {
		t.Fatal("non-exit error should not match")
	}
}

func TestQuote(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "plain", in: "hello", want: "hello"},
		{name: "space", in: "hello world", want: `"hello world"`},
		{name: "tab", in: "hello\tworld", want: `"hello	world"`},
		{name: "newline", in: "hello\nworld", want: "\"hello\nworld\""},
		{name: "hash", in: "value#hash", want: `"value#hash"`},
		{name: "single quote", in: "don't", want: `"don't"`},
		{name: "double quote", in: `say "hi"`, want: `"say \"hi\""`},
		{name: "backslash", in: `C:\tmp\file`, want: `C:\tmp\file`},
		{name: "backslash quoted", in: `C:\Program Files\app`, want: `"C:\\Program Files\\app"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quote(tt.in); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShell(t *testing.T) {
	env := &model.Env{Name: "dev", Vars: []*model.Var{
		{Key: "PORT", Value: "8080"},
		{Key: "MSG", Value: "it's alive"},
		{Key: "EMPTY", Value: ""},
		{Key: "MULTI", Value: "a\nb"},
	}}
	want := "export EMPTY=''\n" +
		"export MSG='it'\\''s alive'\n" +
		"export MULTI='a\nb'\n" +
		"export PORT='8080'\n"
	if got := Shell(env); got != want {
		t.Fatalf("Shell() = %q, want %q", got, want)
	}
}

func TestShellEmptyEnv(t *testing.T) {
	if got := Shell(&model.Env{}); got != "" {
		t.Fatalf("Shell(empty) = %q, want empty", got)
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "assert" }
