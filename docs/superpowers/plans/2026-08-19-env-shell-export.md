# `envii env` (eval-able shell output) Implementation Plan

> **For agentic workers:** Implement task-by-task. Steps use checkbox (`- [ ]`) syntax. Follow TDD: failing test first. Do not commit unless the human says so.

**Goal:** Add `envii env <project> <env>` that prints one `export KEY='value'` line per variable to stdout, so users can run `eval "$(envii env my-api dev)"` to load an environment into their current shell.

**Architecture:** One new cobra command in `internal/cli` reusing `loadVault()` and `resolveEnv()`. One new pure renderer in `internal/runner` (`Shell`) next to `Dotenv`, with a POSIX single-quote escaper. Output target: POSIX-compatible shells (bash/zsh/sh). Fish is explicitly out of scope.

**Tech Stack:** Go, cobra, stdlib `testing`. No new dependencies.

## Global Constraints

- Packages touched: `internal/runner`, `internal/cli` only
- No new dependencies
- No TUI changes
- Output lines sorted by key (same ordering as `Dotenv`) for deterministic output
- Escape strategy: wrap every value in single quotes, replace each `'` with `'\''` — always correct in POSIX shells, no heuristics
- Line format: `export KEY='<escaped>'`
- All output on `defaultIO.Stdout()`; errors on stderr via cobra

---

## File Structure

### Modify
- `internal/runner/runner.go` — add `Shell(env)` + `shellQuote(s)`
- `internal/cli/commands.go` — add `envCmd()`; register in `internal/cli/cli.go`

### Modify (tests)
- `internal/runner/runner_test.go`
- `internal/cli/cli_more_test.go` (or `commands_test.go`)

---

## Tasks

### Task 1: `runner.Shell` renderer

**Files:** `internal/runner/runner.go`, `internal/runner/runner_test.go`

Add to `internal/runner/runner.go` after `Dotenv`:

```go
// Shell renders an env as POSIX shell export statements, sorted by key.
// Suitable for: eval "$(envii env project envname)"
func Shell(env *model.Env) string {
	vars := make([]*model.Var, len(env.Vars))
	copy(vars, env.Vars)
	sort.Slice(vars, func(i, j int) bool { return vars[i].Key < vars[j].Key })

	var b strings.Builder
	for _, v := range vars {
		b.WriteString("export ")
		b.WriteString(v.Key)
		b.WriteString("=")
		b.WriteString(shellQuote(v.Value))
		b.WriteString("\n")
	}
	return b.String()
}

// shellQuote single-quotes a value, escaping embedded single quotes as '\''.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
```

- [ ] **Step 1: Write failing tests**

```go
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
```

- [ ] **Step 2:** `go test ./internal/runner/` — must fail.
- [ ] **Step 3:** Implement, rerun — must pass.

---

### Task 2: `envCmd` + registration

**Files:** `internal/cli/commands.go`, `internal/cli/cli.go`, test file

Behavior spec:

| Input | Result |
|---|---|
| `envii env api dev` | stdout: `export PORT='8080'\nexport TOKEN='s3cr3t'\n`, exit 0 |
| missing project/env | same errors as `export` command |
| wrong arg count | cobra arg error |

Add to `internal/cli/commands.go` after `exportCmd`:

```go
// envCmd: envii env <project> <env>  (eval-able export lines)
func envCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env <project> <env>",
		Short: "Print shell export statements for an env (eval-able)",
		Example: `  eval "$(envii env my-api dev)"
  eval "$(envii env my-api prod)" && ./run-migrations`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			v, _, _, err := loadVault()
			if err != nil {
				return err
			}
			env, err := resolveEnv(v, args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Fprint(defaultIO.Stdout(), runner.Shell(env))
			return nil
		},
	}
	return cmd
}
```

Register in `internal/cli/cli.go`:

```go
	root.AddCommand(runCmd(), exportCmd(), importCmd(), getCmd(), envCmd())
```

(`getCmd()` only exists if the `get` plan landed first; if not, register without it.)

- [ ] **Step 1: Write failing tests**

```go
func TestEnvCmd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.age")
	withVaultPath(t, path)
	seedVault(t, path, "pw", sampleVault())

	io := &fakeIO{env: map[string]string{"ENVII_PASSPHRASE": "pw"}}
	withIO(t, io)

	err := envCmd().RunE(nil, []string{"api", "dev"})
	if err != nil {
		t.Fatal(err)
	}
	want := "export PORT='8080'\nexport TOKEN='s3cr3t'\n"
	if got := io.stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}
```

Match the exact invocation pattern used by existing command tests in `cli_more_test.go`.

- [ ] **Step 2:** `go test ./internal/cli/` — must fail.
- [ ] **Step 3:** Implement + register, rerun — must pass.

---

### Task 3: Docs

- [ ] Add `### Load an env into your shell` section to `README.md` under **Usage** with the `eval "$(envii env my-api dev)"` example and a one-line note: works with POSIX shells (bash/zsh).

### Task 4: Final verification

- [ ] `go build ./...`
- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] Manual smoke (optional): `eval "$(./envii env ...)"` then `echo $KEY` in bash.

## Done When

- Output round-trips through `eval` correctly, including values containing single quotes, whitespace, and newlines.
- Output is sorted by key.
- All tests pass; README documents the command.
