# 2026-07-16 Non-TUI Testability Foundation Design

## Goal

Improve engineering quality for `envii` by making non-TUI packages highly testable and increasing test coverage to 90%++ using table-driven tests.

Scope:
- `internal/model`
- `internal/store`
- `internal/crypto`
- `internal/runner`
- `internal/cli`

Out of scope:
- `internal/tui`

Constraints:
- Table-driven tests only
- Testable refactor first
- Mocking with `mockery`
- No behavior change in foundation PR
- No commits without user approval

## Current State

Current non-TUI coverage: **33.9%** total.

Testability bottlenecks:
- `internal/store` uses `os.ReadFile`, `os.WriteFile`, `os.Rename`, `os.Stat`, `os.MkdirAll`, `os.UserConfigDir`
- `internal/model` uses `time.Now()`
- `internal/runner` uses `exec.Command` + direct `os.Stdin/Stdout/Stderr`
- `internal/cli` uses `os.Getenv`, `os.Stdin`, `term.ReadPassword`, `os.Open`, `os.WriteFile`, `fmt.Fprintf(os.Stderr)`

## Design Decisions

### Testing Philosophy
- Testable refactor before writing extensive tests
- Table-driven tests without exceptions
- Black-box package testing where possible
- Mock only what is necessary for isolation
- Keep production code simple; put complexity in test layer

### Abstraction Strategy
- Lightweight interface injection per package
- Small centralized test helpers in `internal/testutil`
- Use `mockery` for mock generation

## Foundation Interfaces

### `internal/model`
```go
type Clock interface {
    Now() time.Time
}
```
- Production default: `time.Now`
- Test: fake clock

### `internal/store`
```go
type FS interface {
    UserConfigDir() (string, error)
    Stat(name string) (os.FileInfo, error)
    ReadFile(name string) ([]byte, error)
    WriteFile(name string, data []byte, perm os.FileMode) error
    MkdirAll(path string, perm os.FileMode) error
    Rename(oldpath, newpath string) error
}
```
- Production default: wrapper around `os`
- Test: in-memory or temp-dir fake

### `internal/crypto`
- Mostly pure logic already
- Focus on test vectors and table-driven cases
- No major new abstraction required

### `internal/runner`
```go
type Executor interface {
    Run(argv []string, env []string, stdin io.Reader, stdout, stderr io.Writer) error
}
```
- Production default: `exec.Command` + OS pipe
- Test: fake executor with record/return behavior

### `internal/cli`
```go
type CLIIO interface {
    Getenv(key string) string
    Stdin() io.Reader
    Stdout() io.Writer
    Stderr() io.Writer
    ReadPassword(fd int) ([]byte, error)
}
```
- Production default: OS-backed implementation
- Test: buffer + stub password reader

## File Placement

### Interfaces
- `internal/model/clock.go`
- `internal/store/fs.go`
- `internal/runner/executor.go`
- `internal/cli/io.go`

### Generated Mocks
- `internal/model/mock_clock.go`
- `internal/store/mock_fs.go`
- `internal/runner/mock_executor.go`
- `internal/cli/mock_io.go`
- Use `.mockery.yaml` at repo root for consistent generation
- Exclude generated mock files from coverage calculation

### Test Helpers
- Location: `internal/testutil`
- Keep minimal
- Include only frequently reused helpers:
  - temp dir helper
  - fake clock util
  - buffer IO helper
- If helper only used in one package, keep it in that package's `_test.go`

## Rollout Plan

### PR 1 — Foundation
- Add interfaces + dependency injection
- Add generated mocks
- Add `internal/testutil` basics
- Ensure no behavior change
- Keep commit limited to structural prep

### PR 2 — Core Logic
- Focus on `internal/model` + `internal/store`
- Add table-driven tests
- Raise coverage significantly in these two packages

### PR 3 — Execution Path
- Add `internal/crypto` test vectors
- Add `internal/runner` tests using fake executor

### PR 4 — CLI Layer
- Add `internal/cli` tests using fake IO
- Reach 90%++ coverage across all non-TUI packages

## Coverage Targets

Final targets:
- `internal/model` ≥ 90%
- `internal/store` ≥ 90%
- `internal/crypto` ≥ 90%
- `internal/runner` ≥ 90%
- `internal/cli` ≥ 90%
- `internal/tui` out of scope

## Acceptance Criteria

### Foundation PR
- All new interfaces exist in correct packages
- Production default implementations preserve existing behavior
- Mocks can be generated with `mockery`
- `go test ./...` passes
- `go vet ./...` clean
- No breaking change to CLI/command behavior

### Final Coverage PRs
- All target packages ≥ 90% coverage
- Coverage excludes generated mocks
- All tests use table-driven style
- PR description includes before/after coverage per package

## Verification Checklist

Before merge:
- `go test ./...`
- `go vet ./...`
- Per-package coverage report
- Manual smoke test:
  - `envii`
  - `envii run`
  - `envii export`
  - `envii import`
- Confirm no behavior regression

## Non-Goals

- TUI test coverage in this cycle
- Full rewrite of packages
- Unnecessary abstraction layers
- Over-generalized utility package

## Risks

- Over-abstraction could add complexity
- CLI layer may need incremental refactor due to interactive IO
- High coverage target may expose hidden edge cases requiring additional fixes

## Mitigations

- Keep interfaces small and package-specific
- Refactor only where testability requires it
- Use incremental PR rollout
- Keep `internal/testutil` minimal
