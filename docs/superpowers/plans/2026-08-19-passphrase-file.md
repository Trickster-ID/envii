# `--passphrase-file` Implementation Plan

> **For agentic workers:** Implement task-by-task. Steps use checkbox (`- [ ]`) syntax. Follow TDD: failing test first. Do not commit unless the human says so.

**Goal:** Add a `--passphrase-file <path>` flag so CI jobs can supply the vault passphrase from a secret file instead of an environment variable. `ENVII_PASSPHRASE` already exists (`internal/cli/cli.go` `promptPassphrase`); this plan adds the file-based option beside it.

**Architecture:** One new root-level persistent flag storing into a package var `passphraseFile`. `promptPassphrase` resolves in this order: `ENVII_PASSPHRASE` env var → `--passphrase-file` content → interactive prompt. Only trailing line breaks (`\n`, `\r\n`) are stripped from the file content; no other trimming.

**Tech Stack:** Go, cobra, stdlib. No new dependencies.

## Global Constraints

- Packages touched: `internal/cli` only
- No new dependencies
- Precedence is fixed: env var wins over flag, flag wins over prompt
- Read via `defaultIO` so tests can inject a fake filesystem — add `ReadFile(path string) ([]byte, error)` to the `IO` interface
- Never log or echo the passphrase anywhere
- Error on: unreadable file, empty file after stripping newlines
- Security doc note: prefer this over env vars in CI (env vars can leak into logs/ps); still document that file should be `0600`

---

## File Structure

### Modify
- `internal/cli/io.go` — add `ReadFile` to `IO`; implement on `OSIO` via `os.ReadFile`
- `internal/cli/cli.go` — add `passphraseFile` var + persistent flag; use in `promptPassphrase`
- `internal/cli/cli_more_test.go` — extend `fakeIO` with `ReadFile`; new tests
- `README.md` — document

---

## Tasks

### Task 1: `IO.ReadFile`

**Files:** `internal/cli/io.go`, `internal/cli/mock_io.go`, `internal/cli/cli_more_test.go`

Extend the interface:

```go
type IO interface {
	Getenv(key string) string
	Stdin() io.Reader
	Stdout() io.Writer
	Stderr() io.Writer
	ReadPassword(fd int) ([]byte, error)
	ReadFile(path string) ([]byte, error)
}
```

`OSIO` impl:

```go
func (OSIO) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
```

Update `mock_io.go` and any `fakeIO` used in tests to satisfy the interface (mockery pattern used in this repo — mimic the existing methods in `mock_io.go`).

- [ ] **Step 1:** Add `ReadFile` to the interface only; `go build ./...` must fail (missing impls). That is the failing step.
- [ ] **Step 2:** Implement on `OSIO`, mock, and `fakeIO`; build passes. `go test ./...` still green.

---

### Task 2: Flag + `promptPassphrase` precedence

**Files:** `internal/cli/cli.go`

Add near `vaultPath`:

```go
var passphraseFile string
```

In `Execute` beside the `--vault` flag:

```go
	root.PersistentFlags().StringVar(&passphraseFile, "passphrase-file", "", "read vault passphrase from this file")
```

Rewrite `promptPassphrase` head:

```go
func promptPassphrase(label string) (string, error) {
	if p := defaultIO.Getenv("ENVII_PASSPHRASE"); p != "" {
		return p, nil
	}
	if passphraseFile != "" {
		b, err := defaultIO.ReadFile(passphraseFile)
		if err != nil {
			return "", fmt.Errorf("read passphrase file: %w", err)
		}
		p := strings.TrimRight(string(b), "\r\n")
		if p == "" {
			return "", errors.New("passphrase file is empty")
		}
		return p, nil
	}
	// ...existing interactive prompt code unchanged
}
```

(`strings` import may need adding; `errors` is already imported.)

- [ ] **Step 1: Write failing tests**

```go
func TestPromptPassphraseFile(t *testing.T) {
	tests := []struct {
		name      string
		fileVar   string   // passphraseFile value
		env       map[string]string
		fileData  []byte   // what fakeIO.ReadFile returns
		fileErr   error
		wantPass  string
		wantErr   string
	}{
		{"file basic", "/pw", nil, []byte("hunter2\n"), nil, "hunter2", ""},
		{"file crlf", "/pw", nil, []byte("hunter2\r\n"), nil, "hunter2", ""},
		{"env var wins over file", "/pw", map[string]string{"ENVII_PASSPHRASE": "fromenv"}, []byte("fromfile"), nil, "fromenv", ""},
		{"file error", "/pw", nil, nil, errors.New("boom"), "", "read passphrase file"},
		{"empty file", "/pw", nil, []byte("\n"), nil, "", "passphrase file is empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeIO{env: tt.env}
			// set f's ReadFile behaviour per test (extend fakeIO with readFileData/readFileErr fields)
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
		})
	}
}
```

Extend `fakeIO` with:

```go
type fakeIO struct {
	// ...existing fields
	readFileData []byte
	readFileErr  error
	readFileCalls []string
}

func (f *fakeIO) ReadFile(path string) ([]byte, error) {
	f.readFileCalls = append(f.readFileCalls, path)
	return f.readFileData, f.readFileErr
}
```

- [ ] **Step 2:** `go test ./internal/cli/` — must fail.
- [ ] **Step 3:** Implement, rerun — must pass.

---

### Task 3: End-to-end command test

One test proving a real command (`export`) works with `--passphrase-file`:

- [ ] Seed a vault file, set `passphraseFile` to a path, `fakeIO.ReadFile` returns `"pw\n"`, run `exportCmd().RunE(...)` — expect dotenv on stdout. Model it on the existing command tests in `cli_more_test.go`.

---

### Task 4: Docs

- [ ] `README.md`: under **Usage** (or a new **Non-interactive use / CI** subsection) document all three auth modes, in precedence order:
  1. `ENVII_PASSPHRASE` env var
  2. `--passphrase-file <path>` (recommended for CI; keep the file `0600`)
  3. interactive prompt

### Task 5: Final verification

- [ ] `go build ./...`
- [ ] `go test ./...`
- [ ] `go vet ./...`

## Done When

- `envii export api dev --passphrase-file /secret/pw` works non-interactively with the passphrase never echoed.
- Env var still takes precedence when both are set.
- Empty/unreadable file gives a clear error.
