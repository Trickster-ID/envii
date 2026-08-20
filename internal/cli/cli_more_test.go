package cli

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/trickylab/envii/internal/crypto"
	"github.com/trickylab/envii/internal/model"
	"github.com/trickylab/envii/internal/store"
)

func init() { crypto.SetWorkFactor(10) }

type fakeIO struct {
	env     map[string]string
	stdin   *bytes.Buffer
	stdout  *bytes.Buffer
	stderr  *bytes.Buffer
	passes  [][]byte
	passIdx int
	passErr error

	readFileData  []byte
	readFileErr   error
	readFileCalls []string
}

func (f *fakeIO) Getenv(k string) string {
	if f.env == nil {
		return ""
	}
	return f.env[k]
}
func (f *fakeIO) Stdin() io.Reader  { return f.stdin }
func (f *fakeIO) Stdout() io.Writer { return f.stdout }
func (f *fakeIO) Stderr() io.Writer { return f.stderr }
func (f *fakeIO) ReadFile(path string) ([]byte, error) {
	f.readFileCalls = append(f.readFileCalls, path)
	return f.readFileData, f.readFileErr
}
func (f *fakeIO) ReadPassword(fd int) ([]byte, error) {
	if f.passErr != nil {
		return nil, f.passErr
	}
	if f.passIdx >= len(f.passes) {
		return []byte(""), nil
	}
	p := f.passes[f.passIdx]
	f.passIdx++
	return p, nil
}

func withIO(t *testing.T, io IO) {
	t.Helper()
	prev := defaultIO
	SetIO(io)
	t.Cleanup(func() { SetIO(prev) })
}

func withVaultPath(t *testing.T, path string) {
	t.Helper()
	prev := vaultPath
	vaultPath = path
	t.Cleanup(func() { vaultPath = prev })
}

func withExit(t *testing.T) *int {
	t.Helper()
	code := -1
	prev := exitFunc
	exitFunc = func(c int) { code = c }
	t.Cleanup(func() { exitFunc = prev })
	return &code
}

func seedVault(t *testing.T, path, pass string, v *model.Vault) {
	t.Helper()
	s, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(v, pass); err != nil {
		t.Fatal(err)
	}
}

func sampleVault() *model.Vault {
	return &model.Vault{
		Version: 1,
		Projects: []*model.Project{{
			Name: "api",
			Envs: []*model.Env{{
				Name: "dev",
				Vars: []*model.Var{{Key: "PORT", Value: "8080"}, {Key: "TOKEN", Value: "s3cr3t", Secret: true}},
			}},
		}},
	}
}

func TestPromptPassphrase(t *testing.T) {
	tests := []struct {
		name    string
		io      *fakeIO
		want    string
		wantErr string
		wantOut string
	}{
		{
			name: "from env",
			io:   &fakeIO{env: map[string]string{"ENVII_PASSPHRASE": "from-env"}, stderr: &bytes.Buffer{}},
			want: "from-env",
		},
		{
			name:    "from ReadPassword",
			io:      &fakeIO{passes: [][]byte{[]byte("typed")}, stderr: &bytes.Buffer{}, stdin: &bytes.Buffer{}},
			want:    "typed",
			wantOut: "Passphrase: ",
		},
		{
			name:    "ReadPassword error",
			io:      &fakeIO{passErr: errors.New("no tty"), stderr: &bytes.Buffer{}, stdin: &bytes.Buffer{}},
			wantErr: "read passphrase",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.io.stdout == nil {
				tt.io.stdout = &bytes.Buffer{}
			}
			if tt.io.stderr == nil {
				tt.io.stderr = &bytes.Buffer{}
			}
			if tt.io.stdin == nil {
				tt.io.stdin = &bytes.Buffer{}
			}
			withIO(t, tt.io)
			got, err := promptPassphrase("Passphrase: ")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err=%v want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
			if tt.wantOut != "" && !strings.Contains(tt.io.stderr.String(), tt.wantOut) {
				t.Fatalf("stderr %q want containing %q", tt.io.stderr.String(), tt.wantOut)
			}
		})
	}
}

func TestPromptPassphraseFile(t *testing.T) {
	tests := []struct {
		name     string
		fileVar  string
		env      map[string]string
		fileData []byte
		fileErr  error
		wantPass string
		wantErr  string
	}{
		{"file basic", "/pw", nil, []byte("hunter2\n"), nil, "hunter2", ""},
		{"file crlf", "/pw", nil, []byte("hunter2\r\n"), nil, "hunter2", ""},
		{"env var wins over file", "/pw", map[string]string{"ENVII_PASSPHRASE": "fromenv"}, []byte("fromfile"), nil, "fromenv", ""},
		{"file error", "/pw", nil, nil, errors.New("boom"), "", "read passphrase file"},
		{"empty file", "/pw", nil, []byte("\n"), nil, "", "passphrase file is empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeIO{
				env:          tt.env,
				readFileData: tt.fileData,
				readFileErr:  tt.fileErr,
				stdin:        &bytes.Buffer{},
				stdout:       &bytes.Buffer{},
				stderr:       &bytes.Buffer{},
			}
			withIO(t, f)
			prev := passphraseFile
			passphraseFile = tt.fileVar
			t.Cleanup(func() { passphraseFile = prev })

			got, err := promptPassphrase("Passphrase: ")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.wantPass {
				t.Fatalf("pass = %q, want %q", got, tt.wantPass)
			}
			if tt.env != nil {
				if len(f.readFileCalls) != 0 {
					t.Fatalf("readFileCalls = %v, want 0 calls (env must win)", f.readFileCalls)
				}
			} else if len(f.readFileCalls) != 1 {
				t.Fatalf("readFileCalls = %v, want 1 call", f.readFileCalls)
			}
		})
	}
}

func TestPromptNewPassphrase(t *testing.T) {
	tests := []struct {
		name    string
		passes  [][]byte
		env     map[string]string
		want    string
		wantErr string
	}{
		{
			name:   "match",
			passes: [][]byte{[]byte("abc"), []byte("abc")},
			want:   "abc",
		},
		{
			name:    "mismatch",
			passes:  [][]byte{[]byte("abc"), []byte("xyz")},
			wantErr: "do not match",
		},
		{
			name:    "empty",
			passes:  [][]byte{[]byte(""), []byte("")},
			wantErr: "cannot be empty",
		},
		{
			name: "via env twice",
			env:  map[string]string{"ENVII_PASSPHRASE": "envpass"},
			want: "envpass",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fio := &fakeIO{
				env:    tt.env,
				passes: tt.passes,
				stdin:  &bytes.Buffer{},
				stdout: &bytes.Buffer{},
				stderr: &bytes.Buffer{},
			}
			withIO(t, fio)
			got, err := promptNewPassphrase()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err=%v want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestLoadVault(t *testing.T) {
	pass := "secret"
	t.Run("ok", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "vault.age")
		seedVault(t, path, pass, sampleVault())
		withVaultPath(t, path)
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		v, s, p, err := loadVault()
		if err != nil {
			t.Fatal(err)
		}
		if p != pass {
			t.Fatalf("pass %q", p)
		}
		if s.Path != path {
			t.Fatalf("path %q", s.Path)
		}
		if v.FindProject("api") == nil {
			t.Fatal("missing project")
		}
	})
	t.Run("missing vault", func(t *testing.T) {
		withVaultPath(t, filepath.Join(t.TempDir(), "nope.age"))
		_, _, _, err := loadVault()
		if err == nil || !strings.Contains(err.Error(), "no vault found") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("bad passphrase", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "vault.age")
		seedVault(t, path, pass, sampleVault())
		withVaultPath(t, path)
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": "wrong"},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		_, _, _, err := loadVault()
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("prompt error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "vault.age")
		seedVault(t, path, pass, sampleVault())
		withVaultPath(t, path)
		withIO(t, &fakeIO{
			passErr: errors.New("tty"),
			stdin:   &bytes.Buffer{},
			stdout:  &bytes.Buffer{},
			stderr:  &bytes.Buffer{},
		})
		_, _, _, err := loadVault()
		if err == nil || !strings.Contains(err.Error(), "read passphrase") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestExportCmd(t *testing.T) {
	pass := "secret"
	path := filepath.Join(t.TempDir(), "vault.age")
	seedVault(t, path, pass, sampleVault())
	withVaultPath(t, path)

	t.Run("stdout", func(t *testing.T) {
		out := &bytes.Buffer{}
		errBuf := &bytes.Buffer{}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: out,
			stderr: errBuf,
		})
		cmd := exportCmd()
		cmd.SetArgs([]string{"api", "dev"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "PORT=8080") {
			t.Fatalf("stdout %q", out.String())
		}
	})

	t.Run("file", func(t *testing.T) {
		outFile := filepath.Join(t.TempDir(), ".env")
		errBuf := &bytes.Buffer{}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: errBuf,
		})
		cmd := exportCmd()
		cmd.SetArgs([]string{"api", "dev", "-o", outFile})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "TOKEN=s3cr3t") {
			t.Fatalf("file %q", data)
		}
		if !strings.Contains(errBuf.String(), "wrote") {
			t.Fatalf("stderr %q", errBuf.String())
		}
	})

	t.Run("missing project", func(t *testing.T) {
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		cmd := exportCmd()
		cmd.SetArgs([]string{"nope", "dev"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no vault", func(t *testing.T) {
		withVaultPath(t, filepath.Join(t.TempDir(), "missing.age"))
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		cmd := exportCmd()
		cmd.SetArgs([]string{"api", "dev"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestExportCmdPassphraseFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	seedVault(t, path, "pw", sampleVault())
	withVaultPath(t, path)

	out := &bytes.Buffer{}
	withIO(t, &fakeIO{
		readFileData: []byte("pw\n"),
		stdin:        &bytes.Buffer{},
		stdout:       out,
		stderr:       &bytes.Buffer{},
	})
	prev := passphraseFile
	passphraseFile = "/secret/pw"
	t.Cleanup(func() { passphraseFile = prev })

	cmd := exportCmd()
	cmd.SetArgs([]string{"api", "dev"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "PORT=8080") {
		t.Fatalf("stdout %q", out.String())
	}
}

func TestRunCmd(t *testing.T) {
	pass := "secret"
	path := filepath.Join(t.TempDir(), "vault.age")
	seedVault(t, path, pass, sampleVault())
	withVaultPath(t, path)

	t.Run("with dash", func(t *testing.T) {
		code := withExit(t)
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		cmd := runCmd()
		cmd.SetArgs([]string{"api", "dev", "--", "true"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if *code != 0 {
			t.Fatalf("exit %d", *code)
		}
	})

	t.Run("legacy no dash", func(t *testing.T) {
		code := withExit(t)
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		cmd := runCmd()
		cmd.SetArgs([]string{"api", "dev", "true"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if *code != 0 {
			t.Fatalf("exit %d", *code)
		}
	})

	t.Run("no command", func(t *testing.T) {
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		cmd := runCmd()
		// only project+env, no command
		cmd.SetArgs([]string{"api", "dev"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing env", func(t *testing.T) {
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		cmd := runCmd()
		cmd.SetArgs([]string{"api", "prod", "--", "true"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestImportCmd(t *testing.T) {
	pass := "secret"
	path := filepath.Join(t.TempDir(), "vault.age")
	seedVault(t, path, pass, sampleVault())
	withVaultPath(t, path)

	t.Run("add new keys", func(t *testing.T) {
		envFile := filepath.Join(t.TempDir(), ".env")
		if err := os.WriteFile(envFile, []byte("NEW=1\nPORT=9999\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		errBuf := &bytes.Buffer{}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  bytes.NewBufferString("1\n1\n"),
			stdout: &bytes.Buffer{},
			stderr: errBuf,
		})
		cmd := importCmd()
		cmd.SetArgs([]string{"-f", envFile})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(errBuf.String(), "added") {
			t.Fatalf("stderr %q", errBuf.String())
		}
		// reload
		s, _ := store.New(path)
		v, err := s.Load(pass)
		if err != nil {
			t.Fatal(err)
		}
		m := v.FindProject("api").FindEnv("dev").Map()
		if m["NEW"] != "1" {
			t.Fatalf("NEW=%q", m["NEW"])
		}
		if m["PORT"] != "8080" { // skipped without overwrite
			t.Fatalf("PORT=%q", m["PORT"])
		}
	})

	t.Run("overwrite", func(t *testing.T) {
		envFile := filepath.Join(t.TempDir(), ".env")
		if err := os.WriteFile(envFile, []byte("PORT=1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		errBuf := &bytes.Buffer{}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  bytes.NewBufferString("api\ndev\n"),
			stdout: &bytes.Buffer{},
			stderr: errBuf,
		})
		cmd := importCmd()
		cmd.SetArgs([]string{"-f", envFile, "--overwrite"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		s, _ := store.New(path)
		v, _ := s.Load(pass)
		if v.FindProject("api").FindEnv("dev").Map()["PORT"] != "1" {
			t.Fatal("PORT not overwritten")
		}
	})

	t.Run("all skipped no save noise", func(t *testing.T) {
		// re-seed clean
		seedVault(t, path, pass, sampleVault())
		envFile := filepath.Join(t.TempDir(), ".env")
		if err := os.WriteFile(envFile, []byte("PORT=x\nTOKEN=y\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		errBuf := &bytes.Buffer{}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  bytes.NewBufferString("1\n1\n"),
			stdout: &bytes.Buffer{},
			stderr: errBuf,
		})
		cmd := importCmd()
		cmd.SetArgs([]string{"-f", envFile})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(errBuf.String(), "vault unchanged") {
			t.Fatalf("stderr %q", errBuf.String())
		}
	})

	t.Run("missing file flag", func(t *testing.T) {
		cmd := importCmd()
		cmd.SetArgs(nil)
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing file path", func(t *testing.T) {
		cmd := importCmd()
		cmd.SetArgs([]string{"-f", filepath.Join(t.TempDir(), "nope.env")})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("empty dotenv", func(t *testing.T) {
		envFile := filepath.Join(t.TempDir(), ".env")
		if err := os.WriteFile(envFile, []byte("# only comment\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := importCmd()
		cmd.SetArgs([]string{"-f", envFile})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("create project and env by name", func(t *testing.T) {
		seedVault(t, path, pass, sampleVault())
		envFile := filepath.Join(t.TempDir(), ".env")
		if err := os.WriteFile(envFile, []byte("X=1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		errBuf := &bytes.Buffer{}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  bytes.NewBufferString("web\nstaging\n"),
			stdout: &bytes.Buffer{},
			stderr: errBuf,
		})
		cmd := importCmd()
		cmd.SetArgs([]string{"-f", envFile})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		s, _ := store.New(path)
		v, _ := s.Load(pass)
		if v.FindProject("web") == nil || v.FindProject("web").FindEnv("staging") == nil {
			t.Fatal("web/staging not created")
		}
	})
}

func TestExecuteHelpAndVersion(t *testing.T) {
	// Help/version paths exercise Execute + command tree without TUI.
	t.Run("help", func(t *testing.T) {
		// cobra uses os.Args; Execute() reads them
		old := os.Args
		t.Cleanup(func() { os.Args = old })
		os.Args = []string{"envii", "--help"}
		if err := Execute("test"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("version", func(t *testing.T) {
		old := os.Args
		t.Cleanup(func() { os.Args = old })
		os.Args = []string{"envii", "--version"}
		if err := Execute("1.2.3"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("export subcommand via root", func(t *testing.T) {
		pass := "secret"
		path := filepath.Join(t.TempDir(), "vault.age")
		seedVault(t, path, pass, sampleVault())
		old := os.Args
		t.Cleanup(func() { os.Args = old })
		os.Args = []string{"envii", "--vault", path, "export", "api", "dev"}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		if err := Execute("test"); err != nil {
			t.Fatal(err)
		}
	})
}

func TestOSIOBasics(t *testing.T) {
	var io OSIO
	_ = io.Getenv("PATH")
	if io.Stdin() == nil || io.Stdout() == nil || io.Stderr() == nil {
		t.Fatal("stdio nil")
	}
}

func TestPromptNewPassphraseFirstError(t *testing.T) {
	withIO(t, &fakeIO{
		passErr: errors.New("fail"),
		stdin:   &bytes.Buffer{},
		stdout:  &bytes.Buffer{},
		stderr:  &bytes.Buffer{},
	})
	_, err := promptNewPassphrase()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPromptNewPassphraseSecondError(t *testing.T) {
	// first ok via env, second fails — but env returns same for both calls
	// use passes: first ok, second error via custom IO
	fio := &fakeIO{
		passes:  [][]byte{[]byte("one")},
		passErr: nil,
		stdin:   &bytes.Buffer{},
		stdout:  &bytes.Buffer{},
		stderr:  &bytes.Buffer{},
	}
	// After first pass consumed, second ReadPassword returns empty then we need error
	// Use wrapper:
	withIO(t, &seqIO{
		reads:  []readResult{{b: []byte("a"), err: nil}, {err: errors.New("second fail")}},
		stderr: &bytes.Buffer{},
		stdin:  &bytes.Buffer{},
		stdout: &bytes.Buffer{},
	})
	_, err := promptNewPassphrase()
	if err == nil || !strings.Contains(err.Error(), "read passphrase") {
		t.Fatalf("err=%v", err)
	}
	_ = fio
}

type readResult struct {
	b   []byte
	err error
}

type seqIO struct {
	reads  []readResult
	idx    int
	stdin  *bytes.Buffer
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func (s *seqIO) Getenv(string) string                 { return "" }
func (s *seqIO) Stdin() io.Reader                     { return s.stdin }
func (s *seqIO) Stdout() io.Writer                    { return s.stdout }
func (s *seqIO) Stderr() io.Writer                    { return s.stderr }
func (s *seqIO) ReadFile(path string) ([]byte, error) { return nil, errors.New("unused") }
func (s *seqIO) ReadPassword(int) ([]byte, error) {
	if s.idx >= len(s.reads) {
		return nil, errors.New("no more")
	}
	r := s.reads[s.idx]
	s.idx++
	return r.b, r.err
}

func TestRunTUI(t *testing.T) {
	pass := "secret"
	t.Run("existing vault", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "vault.age")
		seedVault(t, path, pass, sampleVault())
		withVaultPath(t, path)
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		prev := runProgram
		var called bool
		runProgram = func(m tea.Model) error {
			called = true
			if m == nil {
				t.Fatal("nil model")
			}
			return nil
		}
		t.Cleanup(func() { runProgram = prev })
		if err := runTUI(nil, nil); err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatal("runProgram not called")
		}
	})
	t.Run("create new vault", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "new.age")
		withVaultPath(t, path)
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		prev := runProgram
		runProgram = func(m tea.Model) error { return nil }
		t.Cleanup(func() { runProgram = prev })
		if err := runTUI(nil, nil); err != nil {
			t.Fatal(err)
		}
		s, err := store.New(path)
		if err != nil {
			t.Fatal(err)
		}
		if !s.Exists() {
			t.Fatal("vault not created")
		}
	})
	t.Run("prompt error existing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "vault.age")
		seedVault(t, path, pass, sampleVault())
		withVaultPath(t, path)
		withIO(t, &fakeIO{
			passErr: errors.New("tty"),
			stdin:   &bytes.Buffer{},
			stdout:  &bytes.Buffer{},
			stderr:  &bytes.Buffer{},
		})
		if err := runTUI(nil, nil); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("load error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "vault.age")
		seedVault(t, path, pass, sampleVault())
		withVaultPath(t, path)
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": "wrong"},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		if err := runTUI(nil, nil); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("new vault passphrase mismatch", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "new.age")
		withVaultPath(t, path)
		withIO(t, &fakeIO{
			passes: [][]byte{[]byte("a"), []byte("b")},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		if err := runTUI(nil, nil); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("runProgram error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "vault.age")
		seedVault(t, path, pass, sampleVault())
		withVaultPath(t, path)
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		prev := runProgram
		runProgram = func(m tea.Model) error { return errors.New("tui boom") }
		t.Cleanup(func() { runProgram = prev })
		if err := runTUI(nil, nil); err == nil || !strings.Contains(err.Error(), "tui boom") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestPromptPassphraseOSFileFD(t *testing.T) {
	// Hit *os.File fd branch with real stdin file + fake password reader wrapper.
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	withIO(t, &filePassIO{
		file: f,
		pass: []byte("fd-pass"),
		errW: &bytes.Buffer{},
	})
	got, err := promptPassphrase("x: ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "fd-pass" {
		t.Fatalf("got %q", got)
	}
}

type filePassIO struct {
	file *os.File
	pass []byte
	errW *bytes.Buffer
}

func (f *filePassIO) Getenv(string) string            { return "" }
func (f *filePassIO) Stdin() io.Reader                { return f.file }
func (f *filePassIO) Stdout() io.Writer               { return io.Discard }
func (f *filePassIO) Stderr() io.Writer               { return f.errW }
func (f *filePassIO) ReadFile(string) ([]byte, error) { return nil, errors.New("unused") }
func (f *filePassIO) ReadPassword(fd int) ([]byte, error) {
	if fd < 0 {
		return nil, errors.New("bad fd")
	}
	return f.pass, nil
}

func TestExportWriteError(t *testing.T) {
	pass := "secret"
	path := filepath.Join(t.TempDir(), "vault.age")
	seedVault(t, path, pass, sampleVault())
	withVaultPath(t, path)
	withIO(t, &fakeIO{
		env:    map[string]string{"ENVII_PASSPHRASE": pass},
		stdin:  &bytes.Buffer{},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	})
	// directory as -o path → write fails
	cmd := exportCmd()
	cmd.SetArgs([]string{"api", "dev", "-o", t.TempDir()})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected write error")
	}
}

func TestImportParseError(t *testing.T) {
	pass := "secret"
	path := filepath.Join(t.TempDir(), "vault.age")
	seedVault(t, path, pass, sampleVault())
	withVaultPath(t, path)
	envFile := filepath.Join(t.TempDir(), "bad.env")
	if err := os.WriteFile(envFile, []byte("not a valid line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withIO(t, &fakeIO{
		env:    map[string]string{"ENVII_PASSPHRASE": pass},
		stdin:  &bytes.Buffer{},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	})
	cmd := importCmd()
	cmd.SetArgs([]string{"-f", envFile})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestImportPromptProjectEmpty(t *testing.T) {
	pass := "secret"
	path := filepath.Join(t.TempDir(), "vault.age")
	seedVault(t, path, pass, sampleVault())
	withVaultPath(t, path)
	envFile := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envFile, []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withIO(t, &fakeIO{
		env:    map[string]string{"ENVII_PASSPHRASE": pass},
		stdin:  bytes.NewBufferString("\n"),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	})
	cmd := importCmd()
	cmd.SetArgs([]string{"-f", envFile})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected empty project error")
	}
}

func TestImportPromptEnvEmpty(t *testing.T) {
	pass := "secret"
	path := filepath.Join(t.TempDir(), "vault.age")
	seedVault(t, path, pass, sampleVault())
	withVaultPath(t, path)
	envFile := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envFile, []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withIO(t, &fakeIO{
		env:    map[string]string{"ENVII_PASSPHRASE": pass},
		stdin:  bytes.NewBufferString("api\n\n"),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	})
	cmd := importCmd()
	cmd.SetArgs([]string{"-f", envFile})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected empty env error")
	}
}

func TestRunCmdLoadVaultError(t *testing.T) {
	withVaultPath(t, filepath.Join(t.TempDir(), "missing.age"))
	withIO(t, &fakeIO{
		env:    map[string]string{"ENVII_PASSPHRASE": "x"},
		stdin:  &bytes.Buffer{},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	})
	cmd := runCmd()
	cmd.SetArgs([]string{"api", "dev", "--", "true"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error")
	}
}

func TestOSIOReadPasswordSmoke(t *testing.T) {
	// May fail without TTY; just exercise the method.
	_, err := (OSIO{}).ReadPassword(0)
	_ = err
}

func TestStoreNewErrorPaths(t *testing.T) {
	// Empty vault path + broken config dir → store.New fails in loadVault/runTUI.
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	withVaultPath(t, "")

	t.Run("loadVault", func(t *testing.T) {
		_, _, _, err := loadVault()
		if err == nil {
			t.Skip("store.New did not fail on this platform with empty HOME")
		}
	})
	t.Run("runTUI", func(t *testing.T) {
		err := runTUI(nil, nil)
		if err == nil {
			t.Skip("store.New did not fail on this platform with empty HOME")
		}
	})
}

func TestRunTUISaveError(t *testing.T) {
	// Parent path is a file → MkdirAll/Save fails when creating new vault.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	withVaultPath(t, filepath.Join(blocker, "vault.age"))
	withIO(t, &fakeIO{
		env:    map[string]string{"ENVII_PASSPHRASE": "pass"},
		stdin:  &bytes.Buffer{},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	})
	if err := runTUI(nil, nil); err == nil {
		t.Fatal("expected save error")
	}
}

func TestRunCmdExecError(t *testing.T) {
	pass := "secret"
	path := filepath.Join(t.TempDir(), "vault.age")
	seedVault(t, path, pass, sampleVault())
	withVaultPath(t, path)
	withIO(t, &fakeIO{
		env:    map[string]string{"ENVII_PASSPHRASE": pass},
		stdin:  &bytes.Buffer{},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	})
	_ = withExit(t)
	cmd := runCmd()
	cmd.SetArgs([]string{"api", "dev", "--", "/no/such/envii/command/xyz"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected exec error")
	}
}

func TestExportResolve(t *testing.T) {
	pass := "pw"
	path := filepath.Join(t.TempDir(), "vault.age")
	v := &model.Vault{
		Version: 1,
		Projects: []*model.Project{{
			Name: "api",
			Envs: []*model.Env{
				{Name: "shared", Vars: []*model.Var{{Key: "VAR", Value: "base"}}},
				{Name: "dev", Base: "shared", Vars: []*model.Var{
					{Key: "VAR", Value: "overridden"},
					{Key: "EXTRA", Value: "1"},
				}},
				{Name: "orphan", Base: "ghost", Vars: []*model.Var{{Key: "RAW", Value: "1"}}},
			},
		}},
	}
	seedVault(t, path, pass, v)
	withVaultPath(t, path)

	t.Run("resolve on", func(t *testing.T) {
		out := &bytes.Buffer{}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: out,
			stderr: &bytes.Buffer{},
		})
		cmd := exportCmd()
		if err := cmd.ParseFlags([]string{"--resolve"}); err != nil {
			t.Fatal(err)
		}
		if err := cmd.RunE(cmd, []string{"api", "dev"}); err != nil {
			t.Fatal(err)
		}
		s := out.String()
		if !strings.Contains(s, "EXTRA=1") || !strings.Contains(s, "VAR=overridden") {
			t.Fatalf("stdout %q", s)
		}
	})

	t.Run("resolve off keeps raw env", func(t *testing.T) {
		out := &bytes.Buffer{}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: out,
			stderr: &bytes.Buffer{},
		})
		cmd := exportCmd()
		if err := cmd.RunE(cmd, []string{"api", "orphan"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "RAW=1") {
			t.Fatalf("stdout %q", out.String())
		}
	})

	t.Run("resolve on with missing base errors", func(t *testing.T) {
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: &bytes.Buffer{},
			stderr: &bytes.Buffer{},
		})
		cmd := exportCmd()
		if err := cmd.ParseFlags([]string{"--resolve"}); err != nil {
			t.Fatal(err)
		}
		err := cmd.RunE(cmd, []string{"api", "orphan"})
		if err == nil || !strings.Contains(err.Error(), `base env "ghost" not found`) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestImportSaveError(t *testing.T) {
	pass := "secret"
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")
	seedVault(t, path, pass, sampleVault())
	withVaultPath(t, path)

	envFile := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envFile, []byte("EXTRA=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Make vault dir read-only so Save fails after successful load/import merge.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	withIO(t, &fakeIO{
		env:    map[string]string{"ENVII_PASSPHRASE": pass},
		stdin:  bytes.NewBufferString("1\n1\n"),
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	})
	cmd := importCmd()
	cmd.SetArgs([]string{"-f", envFile})
	if err := cmd.Execute(); err == nil {
		// some OS still allow write by owner; force skip if not enforced
		t.Skip("chmod did not block save on this platform")
	}
}

func TestPromptProjectLineError(t *testing.T) {
	_, err := promptProject(bufio.NewReader(errReader{}), &model.Vault{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPromptEnvLineError(t *testing.T) {
	_, err := promptEnv(bufio.NewReader(errReader{}), &model.Project{Name: "api"})
	if err == nil {
		t.Fatal("expected error")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read fail") }

func TestEnvCmd(t *testing.T) {
	pass := "secret"
	path := filepath.Join(t.TempDir(), "vault.age")
	seedVault(t, path, pass, sampleVault())
	withVaultPath(t, path)

	t.Run("stdout", func(t *testing.T) {
		out := &bytes.Buffer{}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: out,
			stderr: &bytes.Buffer{},
		})
		cmd := envCmd()
		cmd.SetArgs([]string{"api", "dev"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		want := "export PORT='8080'\nexport TOKEN='s3cr3t'\n"
		if got := out.String(); got != want {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	})

	t.Run("missing project", func(t *testing.T) {
		out := &bytes.Buffer{}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: out,
			stderr: &bytes.Buffer{},
		})
		cmd := envCmd()
		cmd.SetArgs([]string{"nope", "dev"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error")
		}
		if out.Len() != 0 {
			t.Fatalf("stdout on error = %q, want empty", out.String())
		}
	})

	t.Run("wrong arg count", func(t *testing.T) {
		out := &bytes.Buffer{}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: out,
			stderr: &bytes.Buffer{},
		})
		cmd := envCmd()
		cmd.SetArgs([]string{"api"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error")
		}
		if out.Len() != 0 {
			t.Fatalf("stdout on error = %q, want empty", out.String())
		}
	})
}

func TestImportLoadVaultError(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envFile, []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withVaultPath(t, filepath.Join(t.TempDir(), "missing.age"))
	withIO(t, &fakeIO{
		env:    map[string]string{"ENVII_PASSPHRASE": "x"},
		stdin:  &bytes.Buffer{},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	})
	cmd := importCmd()
	cmd.SetArgs([]string{"-f", envFile})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected loadVault error")
	}
}
