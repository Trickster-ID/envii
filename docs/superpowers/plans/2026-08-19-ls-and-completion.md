# `envii ls` + shell completion Implementation Plan

> **For agentic workers:** Implement task-by-task. Steps use checkbox (`- [ ]`) syntax. Follow TDD: failing test first. Do not commit unless the human says so.

**Goal:** (1) `envii ls` lists vault contents non-TUI. (2) Tab-completion of project/env/KEY arguments in `get`, `env`, `export`, `run` via cobra `ValidArgsFunction` — so `envii get <TAB>` suggests projects, then envs, then keys.

**Facts about the codebase the plan relies on:**
- cobra is already a dependency; `cobra completion` subcommand already exists (auto-generated). We do NOT write shell scripts manually.
- All commands live in `internal/cli/commands.go` and call `loadVault()` which prompts for a passphrase. Completion must ALSO authenticate (vault is encrypted — suggestion requires decrypting). That means completion shares `loadVault()`; completion only triggers when the user already has the passphrase available (env var / passphrase-file), which is the normal scripted/CI case.
- Completion functions must print nothing but suggestions; cobra writes suggestions itself. Errors from `ValidArgsFunction` are ignored by cobra (fine).

## Scope decisions (do not expand during implementation)

- Output format for `ls`: plain, script-friendly, no colors, no tree characters.
- Default `envii ls` = projects, one per line.
- `envii ls <project>` = envs of that project, one per line (note inherited `base` lines if set: print `name (base: shared)` ONLY when `--long` flag given, otherwise bare names — keep default output pipe-clean).
- `envii ls <project> <env>` = keys, one per line, sorted, secrets marked `KEY*` only with `--long` (bare name by default).
- No `-o json` flag. YAGNI.
- Completion on: `get` (arg1=project, arg2=env, arg3=KEY), `env` (project, env), `export` (project, env), `run` (project, env, then stop — command tail must NOT be completed).
- If `loadVault` fails during completion: return empty list silently (existing cobra behavior for nil suggestions).
- `cobra.NoFileComp` for the KEY arg and shell-arg tail positions.

## Global Constraints

- Packages touched: `internal/cli` only
- No new dependencies
- No TUI changes
- Every suggestion list sorted alphabetically for stable output
- Helpers that list names are pure functions taking `*model.Vault` / `*model.Project` / `*model.Env` — test them directly with synthetic models, no vault file needed for their tests
- Command-level tests use repo helpers from `cli_more_test.go` (`fakeIO`, `withIO`, `withVaultPath`, `seedVault`, `sampleVault`)

---

## File Structure

### Create
- `internal/cli/list.go` — `lsCmd`, pure listing helpers, completion functions
- `internal/cli/list_test.go`

### Modify
- `internal/cli/cli.go` — register `lsCmd`
- `internal/cli/commands.go` — attach `ValidArgsFunction` to `getCmd`, `envCmd`, `exportCmd`, `runCmd` (only commands that exist)

---

## Tasks

### Task 1: Pure listing helpers

**Files:** `internal/cli/list.go`, `internal/cli/list_test.go`

Three pure functions (no flag state):

```go
// projectNames returns sorted project names.
func projectNames(v *model.Vault) []string

// envNames returns sorted env names of a project.
func envNames(p *model.Project) []string

// keyNames returns sorted var keys of an env.
func keyNames(e *model.Env) []string
```

Each: collect, `sort.Strings`, return. ~4 lines each.

- [ ] **Step 1: Write failing table-driven tests** covering: empty collections, multi-element sorting, single element. Use `reflect.DeepEqual` against `[]string` expected values.
- [ ] **Step 2:** `go test ./internal/cli/` — must fail.
- [ ] **Step 3:** Implement, rerun — must pass.

---

### Task 2: `lsCmd`

**Files:** `internal/cli/list.go`, `internal/cli/cli.go`

Behavior spec:

| Invocation | Output (stdout) | Notes |
|---|---|---|
| `envii ls` | `api\nweb\n` | sorted project names |
| `envii ls missing` | error `project "missing" not found` | non-zero exit |
| `envii ls api` | `dev\nprod\n` | sorted env names |
| `envii ls api dev` | `PORT\nTOKEN\n` | sorted keys |
| `envii ls --long api dev` | `PORT\nTOKEN *\n` | exact format: `KEY` then ` *` suffix for Secret vars |
| `envii ls --long api` | `dev (base: shared)\nprod\n` | `(base: X)` rendered ONLY when `Base != ""` |
| `envii ls a b c` (4+ args) | cobra arg error | |

Implementation sketch:

```go
func lsCmd() *cobra.Command {
	var long bool
	cmd := &cobra.Command{
		Use:     "ls [project] [env]",
		Short:   "List projects, environments, or keys",
		Args:    cobra.MaximumNArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			v, _, _, err := loadVault()
			if err != nil {
				return err
			}
			out := defaultIO.Stdout()
			w := tabwriter.NewWriter(out, 0, 0, 0, ' ', 0)  // only if you need alignment; plain Fprintln is fine too
			switch len(args) {
			case 0:
				for _, name := range projectNames(v) {
					fmt.Fprintln(out, name)
				}
				return nil
			case 1:
				p := v.FindProject(args[0])
				if p == nil {
					return fmt.Errorf("project %q not found", args[0])
				}
				for _, name := range envNames(p) {
					if long {
						if e := p.FindEnv(name); e != nil && e.Base != "" {
							fmt.Fprintf(out, "%s (base: %s)\n", name, e.Base)
							continue
						}
					}
					fmt.Fprintln(out, name)
				}
				return nil
			default:
				env, err := resolveEnv(v, args[0], args[1])
				if err != nil {
					return err
				}
				for _, v2 := range env.Vars { // print raw order sorted via keyNames
					// sort-slice Vars instead; see test expectation
				}
				return nil
			}
		},
	}
	cmd.Flags().BoolVarP(&long, "long", "l", false, "show secret markers and base envs")
	return cmd
}
```

Notes for the implementer:
- For the key list, build the lines from `env.Vars` sorted by Key so you can attach the `*` marker in the same pass; `keyNames` alone loses the Secret flag. Keep `keyNames` for the completion path.
- Do NOT introduce tabwriter unless tests demand alignment — plain `Fprintln` is simpler. Delete the tabwriter line above when implementing.
- The trailing `(base: shared)` space before the paren is required by the table above — trim it in tests.

- [ ] **Step 1: Write failing tests** — cover every row of the behavior table. Use `sampleVault()` for the default case; build a custom vault (with a `Base` field set and a Secret var) for the `--long` rows. Seed a vault via `seedVault`, set `ENVII_PASSPHRASE` via fakeIO, call `lsCmd` with flags parsed the same way existing tests do it.
- [ ] **Step 2:** `go test ./internal/cli/` — must fail.
- [ ] **Step 3:** Implement `lsCmd` + register in `cli.go` via `root.AddCommand(lsCmd(), ...)`. Rerun — must pass.

---

### Task 3: Completion wiring

**Files:** `internal/cli/list.go` (completion helpers), `internal/cli/commands.go`

One shared completion factory + per-command wiring:

```go
// completeVault returns suggestions from loadVault for the given positional arg:
// pos 0 = projects, 1 = envs of args[0], 2 = keys of args[0]/args[1].
func completeVault(pos int, needEnvLevel bool) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) != pos {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		v, _, _, err := loadVault()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		switch pos {
		case 0:
			return projectNames(v), cobra.ShellCompDirectiveNoFileComp
		case 1:
			if p := v.FindProject(args[0]); p != nil {
				return envNames(p), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		default: // pos == 2
			if e, err := resolveEnv(v, args[0], args[1]); err == nil {
				return keyNames(e), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}
```

Attach (only to commands that exist in the codebase):

```go
	getCmd:     cmd.ValidArgsFunction = completeVault(2, true)
	envCmd:    cmd.ValidArgsFunction = completeVault(1, false)
	exportCmd: cmd.ValidArgsFunction = completeVault(1, false)
	runCmd:    cmd.ValidArgsFunction = completeVault(1, false) // stops at env; command tail not completed
```

If `getCmd`/`envCmd` from the other plans haven't landed, only wire `exportCmd` and `runCmd` and leave a `// completion: extend when get/env land` comment.

- [ ] **Step 1: Write failing tests**

Test the pure part: seed a vault, fakeIO passphrase, and call the returned functions directly:

```go
func TestCompleteVault(t *testing.T) {
	// path, seedVault("pw", sampleVault()), ENVII_PASSPHRASE="pw" via fakeIO
	tests := []struct {
		name string
		pos  int
		args []string
		want []string
	}{
		{"projects", 0, nil, []string{"api"}},
		{"envs", 1, []string{"api"}, []string{"dev"}},
		{"keys", 2, []string{"api", "dev"}, []string{"PORT", "TOKEN"}},
		{"wrong pos", 0, []string{"api"}, nil}, // pos says 0 but args given → no completion
		{"missing project", 1, []string{"web"}, nil},
	}
	...
}
```

- [ ] **Step 2:** `go test ./internal/cli/` — must fail.
- [ ] **Step 3:** Implement + wire; rerun — must pass.
- [ ] **Step 4:** Run `go build ./cmd/envii && ./envii completion zsh | head` to confirm completion script generation still works.

---

### Task 4: Docs

- [ ] `README.md` under **Usage**:
  - `### List vault contents` documenting the three `ls` forms.
  - `### Shell completion` with:
    ```sh
    echo 'source <(envii completion zsh)' >> ~/.zshrc
    # or for bash:
    echo 'source <(envii completion bash)' >> ~/.bashrc
    ```

### Task 5: Final verification

- [ ] `go build ./...`
- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] Manual smoke (optional): source completion, `envii export <TAB>`.

## Done When

- `ls` default output is one-name-per-line, pipe-clean, sorted.
- `--long` shows secret markers and base envs.
- Completion suggests projects → envs → keys for all applicable commands.
- Completion with a missing/unopenable vault fails silently (empty suggestion list), never prints an error to the user's prompt.
