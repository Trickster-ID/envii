# Non-TUI Testability Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make non-TUI packages in `envii` testable and prepare the repo structure to reach 90%++ coverage using table-driven tests.

**Architecture:** Add small package-level interfaces for OS/IO dependencies, inject them via constructors/helpers, generate mocks with `mockery`, and add a minimal shared test utility package. Production behavior stays unchanged in the foundation phase.

**Tech Stack:** Go, `go test`, `mockery`, `go vet`, standard library testing patterns

## Global Constraints

- Table-driven tests required for all tests added in this plan
- Use `mockery` for mock generation
- No behavior change in production commands
- No commits without explicit user approval in execution session
- Coverage target for final rollout: ≥90% for `internal/model`, `internal/store`, `internal/crypto`, `internal/runner`, `internal/cli`
- Exclude generated mocks from coverage accounting
- `internal/tui` out of scope

---

## File Structure

### Create
- `internal/model/clock.go`
- `internal/store/fs.go`
- `internal/runner/executor.go`
- `internal/cli/io.go`
- `.mockery.yaml`
- `internal/testutil/doc.go`
- `internal/testutil/tempdir.go`
- `internal/testutil/buffer.go`

### Modify (wiring only, no behavior change)
- `internal/model/model.go`
- `internal/store/store.go`
- `internal/runner/runner.go`
- `internal/cli/cli.go`
- `internal/cli/commands.go`

### Generated mocks (created by tooling, not manual code)
- `internal/model/mock_clock.go`
- `internal/store/mock_fs.go`
- `internal/runner/mock_executor.go`
- `internal/cli/mock_io.go`

---

## Tasks

### Task 1: Add mockery config

**Files:**
- Create: `.mockery.yaml`

**Interfaces:**
- Produces: repo-wide mockery configuration consumed by later generation steps

- [ ] **Step 1: Create `.mockery.yaml`**

```yaml
version: 2
mockname: "Mock{{.InterfaceName}}"
filename: "mock_{{.InterfaceName | snakecase}}.go"
outpkg: "{{.PackageName}}"
dir: "{{.InterfaceDir}}"
inpackage: true
```

- [ ] **Step 2: Verify file exists**

Run:
```sh
cat .mockery.yaml
```

Expected:
File contents match above YAML exactly.

- [ ] **Step 3: Commit**

```sh
git add .mockery.yaml
git commit -m "chore: add mockery config"
```

---

### Task 2: Add shared test utilities

**Files:**
- Create: `internal/testutil/doc.go`
- Create: `internal/testutil/tempdir.go`
- Create: `internal/testutil/buffer.go`

**Interfaces:**
- Produces helpers used by future tests:
  - `testutil.TempDir(t *testing.T) string`
  - `testutil.BufferIO() *BufferIO`
  - `BufferIO.Stdin`, `BufferIO.Stdout`, `BufferIO.Stderr`

- [ ] **Step 1: Create `internal/testutil/doc.go`**

```go
// Package testutil contains small shared helpers for tests.
package testutil
```

- [ ] **Step 2: Create `internal/testutil/tempdir.go`**

```go
package testutil

import "testing"

// TempDir returns a temporary directory that is cleaned up after the test.
func TempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}
```

- [ ] **Step 3: Create `internal/testutil/buffer.go`**

```go
package testutil

import "bytes"

// BufferIO holds in-memory stdio buffers for testing.
type BufferIO struct {
	Stdin  *bytes.Buffer
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}

// BufferIO returns a new BufferIO with initialized buffers.
func NewBufferIO() *BufferIO {
	return &BufferIO{
		Stdin:  &bytes.Buffer{},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
}
```

- [ ] **Step 4: Run package tests**

Run:
```sh
go test ./internal/testutil/...
```

Expected:
PASS

- [ ] **Step 5: Commit**

```sh
git add internal/testutil
git commit -m "test: add shared test utilities"
```

---

### Task 3: Add model clock abstraction

**Files:**
- Create: `internal/model/clock.go`
- Modify: `internal/model/model.go`

**Interfaces:**
- Produces: `model.Clock` with `Now() time.Time`
- Wiring: `NewVault` accepts optional `Clock`; default preserves current behavior with `time.Now()`

- [ ] **Step 1: Write failing test for clock injection**

Create `internal/model/clock_test.go`:

```go
package model_test

import (
	"testing"
	"time"

	"github.com/trickylab/envii/internal/model"
)

func TestNewVaultUsesInjectedClock(t *testing.T) {
	fixed := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	c := model.ClockFunc(func() time.Time { return fixed })

	v := model.NewVault(model.WithClock(c))

	if !v.UpdatedAt.Equal(fixed) {
		t.Fatalf("expected UpdatedAt %v, got %v", fixed, v.UpdatedAt)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```sh
go test ./internal/model/... -run TestNewVaultUsesInjectedClock
```

Expected:
Compile failure / missing options support.

- [ ] **Step 3: Add clock abstraction and option wiring**

Create `internal/model/clock.go`:

```go
package model

import "time"

// Clock abstracts time for testability.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts a function to Clock.
type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time { return f() }

type vaultOptions struct {
	clock Clock
}

type VaultOption func(*vaultOptions)

func WithClock(c Clock) VaultOption {
	return func(o *vaultOptions) { o.clock = c }
}
```

Modify `internal/model/model.go` to:
- accept `...VaultOption`
- default to `time.Now()` when no clock supplied
- use injected clock for `UpdatedAt`

- [ ] **Step 4: Run tests**

Run:
```sh
go test ./internal/model/...
```

Expected:
PASS

- [ ] **Step 5: Commit**

```sh
git add internal/model
git commit -m "refactor(model): add clock injection for testability"
```

---

### Task 4: Add store FS abstraction

**Files:**
- Create: `internal/store/fs.go`
- Modify: `internal/store/store.go`

**Interfaces:**
- Produces: `store.FS` interface covering config dir, stat, read, write, mkdir, rename
- Wiring: `Store` accepts optional `FS`; default uses OS implementation

- [ ] **Step 1: Write failing test for injected FS**

Create `internal/store/fs_test.go`:

```go
package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/trickylab/envii/internal/store"
)

func TestStoreLoadAndSaveWithInjectedFS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")
	fs := store.OSFS{}
	s := store.New(store.WithPath(path), store.WithFS(fs))

	v, err := s.LoadOrCreate([]byte("passphrase"))
	if err != nil {
		t.Fatalf("loadOrCreate: %v", err)
	}
	v.Projects = append(v.Projects, nil)

	if err := s.Save(v, []byte("passphrase")); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```sh
go test ./internal/store/... -run TestStoreLoadAndSaveWithInjectedFS
```

Expected:
Compile failure / missing option types.

- [ ] **Step 3: Add FS interface and wiring**

Create `internal/store/fs.go`:

```go
package store

import (
	"os"
)

// FS abstracts filesystem operations for store testability.
type FS interface {
	UserConfigDir() (string, error)
	Stat(name string) (os.FileInfo, error)
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Rename(oldpath, newpath string) error
}

// OSFS implements FS using the real OS.
type OSFS struct{}

func (OSFS) UserConfigDir() (string, error)                 { return os.UserConfigDir() }
func (OSFS) Stat(name string) (os.FileInfo, error)          { return os.Stat(name) }
func (OSFS) ReadFile(name string) ([]byte, error)           { return os.ReadFile(name) }
func (OSFS) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}
func (OSFS) MkdirAll(path string, perm os.FileMode) error   { return os.MkdirAll(path, perm) }
func (OSFS) Rename(oldpath, newpath string) error           { return os.Rename(oldpath, newpath) }
```

Modify `internal/store/store.go` to:
- add `storeOption` + `WithFS(fs FS)` + `WithPath(p string)`
- default FS to `OSFS{}`
- use injected FS in load/save flows

- [ ] **Step 4: Run tests**

Run:
```sh
go test ./internal/store/...
```

Expected:
PASS

- [ ] **Step 5: Commit**

```sh
git add internal/store
git commit -m "refactor(store): add filesystem injection for testability"
```

---

### Task 5: Add runner executor abstraction

**Files:**
- Create: `internal/runner/executor.go`
- Modify: `internal/runner/runner.go`

**Interfaces:**
- Produces: `runner.Executor` with `Run(argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error`
- Wiring: runner command uses injected executor; default preserves current OS execution behavior

- [ ] **Step 1: Write failing test for executor injection**

Create `internal/runner/executor_test.go`:

```go
package runner_test

import (
	"bytes"
	"testing"

	"github.com/trickylab/envii/internal/runner"
)

type fakeExecutor struct {
	called bool
}

func (f *fakeExecutor) Run(argv []string, env []string, stdin interface{ Read([]byte) (int, error) }, stdout, stderr interface{ Write([]byte) (int, error) }) error {
	f.called = true
	_, _ = stdout.Write([]byte("ok"))
	return nil
}

func TestRunWithInjectedExecutor(t *testing.T) {
	fe := &fakeExecutor{}
	out := &bytes.Buffer{}
	r := runner.New(runner.WithExecutor(fe))

	if err := r.Run([]string{"echo"}, []string{"K=V"}, nil, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !fe.called {
		t.Fatalf("expected executor to be called")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```sh
go test ./internal/runner/... -run TestRunWithInjectedExecutor
```

Expected:
Compile failure / missing option support.

- [ ] **Step 3: Add executor abstraction and wiring**

Create `internal/runner/executor.go`:

```go
package runner

import "io"

// Executor abstracts subprocess execution.
type Executor interface {
	Run(argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type runnerOptions struct {
	executor Executor
}

type Option func(*runnerOptions)

func WithExecutor(e Executor) Option {
	return func(o *runnerOptions) { o.executor = e }
}
```

Modify `internal/runner/runner.go` to:
- expose constructor `New(...Option)`
- keep public API behavior equivalent
- route execution through injected executor when provided

- [ ] **Step 4: Run tests**

Run:
```sh
go test ./internal/runner/...
```

Expected:
PASS

- [ ] **Step 5: Commit**

```sh
git add internal/runner
git commit -m "refactor(runner): add executor injection for testability"
```

---

### Task 6: Add CLI IO abstraction

**Files:**
- Create: `internal/cli/io.go`
- Modify: `internal/cli/cli.go`
- Modify: `internal/cli/commands.go`

**Interfaces:**
- Produces: `cli.IO` with `Getenv`, `Stdin`, `Stdout`, `Stderr`, `ReadPassword`
- Wiring: CLI commands use injected IO; default preserves OS behavior

- [ ] **Step 1: Write failing test for IO injection**

Create `internal/cli/io_test.go`:

```go
package cli_test

import (
	"bytes"
	"testing"

	"github.com/trickylab/envii/internal/cli"
)

type testIO struct {
	stdin  *bytes.Buffer
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func (t *testIO) Getenv(k string) string          { return "" }
func (t *testIO) Stdin() interface{ Read([]byte) (int, error) }  { return t.stdin }
func (t *testIO) Stdout() interface{ Write([]byte) (int, error) } { return t.stdout }
func (t *testIO) Stderr() interface{ Write([]byte) (int, error) } { return t.stderr }
func (t *testIO) ReadPassword(fd int) ([]byte, error)            { return []byte("pass"), nil }

func TestIOAbstractionCompiles(t *testing.T) {
	io := &testIO{
		stdin:  &bytes.Buffer{},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
	_ = io
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```sh
go test ./internal/cli/... -run TestIOAbstractionCompiles
```

Expected:
Compile failure / missing abstraction file.

- [ ] **Step 3: Add IO abstraction and wiring**

Create `internal/cli/io.go`:

```go
package cli

import "io"

// IO abstracts CLI environment and stdio for testability.
type IO interface {
	Getenv(key string) string
	Stdin() io.Reader
	Stdout() io.Writer
	Stderr() io.Writer
	ReadPassword(fd int) ([]byte, error)
}
```

Modify `internal/cli/cli.go` and `internal/cli/commands.go` to:
- accept `IO` via options or constructor where feasible
- keep command behavior unchanged
- fallback to current OS-backed behavior when `IO` is nil

- [ ] **Step 4: Run tests**

Run:
```sh
go test ./internal/cli/...
```

Expected:
PASS

- [ ] **Step 5: Commit**

```sh
git add internal/cli
git commit -m "refactor(cli): add IO abstraction for testability"
```

---

### Task 7: Generate mocks and verify tooling

**Files:**
- Modify: generated mock files via tooling

**Interfaces:**
- Consumes: `model.Clock`, `store.FS`, `runner.Executor`, `cli.IO`
- Produces: generated mocks under each package

- [ ] **Step 1: Generate mocks**

Run:
```sh
mockery
```

Expected:
Mock files generated in respective packages.

- [ ] **Step 2: Run full tests**

Run:
```sh
go test ./...
```

Expected:
PASS

- [ ] **Step 3: Run vet**

Run:
```sh
go vet ./...
```

Expected:
No output / clean

- [ ] **Step 4: Commit**

```sh
git add -A
git commit -m "chore: generate mocks for testability interfaces"
```

---

### Task 8: Add foundation verification checks

**Files:**
- Modify: none (verification only)

**Interfaces:**
- Consumes: current repo state after refactor

- [ ] **Step 1: Run tests**

Run:
```sh
go test ./...
```

Expected:
PASS

- [ ] **Step 2: Run vet**

Run:
```sh
go vet ./...
```

Expected:
Clean

- [ ] **Step 3: Smoke check commands still build**

Run:
```sh
go build ./cmd/envii
```

Expected:
Build succeeds

- [ ] **Step 4: Commit only if fix needed**

If any small fix was required:

```sh
git add <changed-files>
git commit -m "fix: stabilize foundation refactor"
```

Otherwise, no commit required.

---

## Self-Review Checklist

- All foundation interfaces exist in correct packages
- Default implementations preserve current behavior
- Mockery generation configured
- Tests pass
- Vet clean
- No unrelated refactors
- No behavioral changes in CLI commands
- Generated mock files excluded from future coverage reporting
