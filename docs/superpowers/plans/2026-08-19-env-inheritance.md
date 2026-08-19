# Env Inheritance Implementation Plan

> **For agentic workers:** Implement task-by-task. Steps use checkbox (`- [ ]`) syntax. Follow TDD: failing test first. Do not commit unless the human says so.

**Goal:** Let an env inherit variables from another env in the same project via a `base` field. Example: `prod` declares `"base": "shared"` → resolved `prod` = `shared` vars overridden by `prod`'s own vars. This removes duplication when `dev`/`staging`/`prod` share 90% of their config.

**Architecture:**

1. **Model:** add `Base string` (JSON `"base,omitempty"`) to `model.Env`. Add two pure functions to `internal/model`:
   - `MergeEnvs(base, override *Env) *Env` — overlay `override.Vars` on top of `base.Vars`; override wins whole (value AND `Secret` flag). Result Name = override.Name. Pure, no mutation of inputs.
   - `(*Project).ResolveEnv(name string) (*Env, error)` — walk the `Base` chain, deepest base first, folding with `MergeEnvs`. Detect cycles and depth > 8 → error.
2. **CLI:** opt-in `--resolve` flag on `get`, `env`, and `export` commands (default `false`). When set, they call `p.ResolveEnv(...)` instead of the plain lookup. `run` stays raw (no `--resolve`) — YAGNI.
3. **No TUI work in this plan.** The TUI keeps editing raw envs. (Users set `base` by hand today; a TUI affordance is a follow-up.)

**Tech Stack:** Go, cobra, stdlib `testing`. No new dependencies. Vault JSON schema gains one optional field — fully backward compatible: old vaults unmarshal fine (`Base` stays `""`).

## Global Constraints

- Packages touched: `internal/model`, `internal/cli` only
- No new dependencies
- No TUI changes
- No vault migration needed (`omitempty` field)
- Cycle/depth limits: max chain depth 8; any cycle is a hard error
- `--resolve` only exists on `get`, `env`, `export` (the `get`/`env` plans are prerequisites — if they haven't landed, only wire `export --resolve`)
- `base` must reference an env in the SAME project; cross-project references are out of scope
- Tests must be table-driven following repo style

---

## File Structure

### Create
- `internal/model/resolve.go` — `Base` is declared in `model.go`; `MergeEnvs` + `ResolveEnv` live here
- `internal/model/resolve_test.go`

### Modify
- `internal/model/model.go` — add `Base` field to `Env` struct
- `internal/cli/commands.go` — add `--resolve` flag + resolution helper
- `internal/cli/cli_more_test.go` — command tests

---

## Tasks

### Task 1: `Env.Base` field

**Files:** `internal/model/model.go`

Change the `Env` struct to:

```go
// Env is a named set of variables (e.g. "dev", "staging", "prod").
// Base optionally names another env in the same project to inherit from.
type Env struct {
	Name  string `json:"name"`
	Base  string `json:"base,omitempty"`
	Vars  []*Var `json:"vars"`
}
```

- [ ] **Step 1: Write failing round-trip test in `internal/model/model_test.go`**

```go
func TestEnvBaseJSONRoundTrip(t *testing.T) {
	raw := []byte(`{"name":"prod","base":"shared","vars":[]}`)
	var e Env
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if e.Base != "shared" {
		t.Fatalf("Base = %q", e.Base)
	}
	out, _ := json.Marshal(&Env{Name: "dev"})
	if string(out) != `{"name":"dev","vars":null}` {
		t.Fatalf("Base must be omitted when empty, got %s", out)
	}
}
```

(Import `encoding/json` in the test file.)

- [ ] **Step 2:** `go test ./internal/model/` — must fail on the marshal check (or compile) only after adding field logic. Run, observe.
- [ ] **Step 3:** Add the field, rerun — must pass. Run `go test ./...` to confirm nothing else breaks.

---

### Task 2: `MergeEnvs`

**Files:** `internal/model/resolve.go` (new), `internal/model/resolve_test.go` (new)

```go
// MergeEnvs overlays override's vars on top of base's vars.
// Keys present in override replace base entries entirely (value and Secret flag).
// Inputs are not mutated; a new Env is returned with override's Name.
func MergeEnvs(base, override *Env) *Env {
	merged := &Env{Name: override.Name, Base: override.Base, Vars: []*Var{}}
	index := make(map[string]int)
	for _, v := range base.Vars {
		cp := *v
		merged.Vars = append(merged.Vars, &cp)
		index[v.Key] = len(merged.Vars) - 1
	}
	for _, v := range override.Vars {
		cp := *v
		if i, ok := index[v.Key]; ok {
			merged.Vars[i] = &cp
		} else {
			merged.Vars = append(merged.Vars, &cp)
			index[v.Key] = len(merged.Vars) - 1
		}
	}
	return merged
}
```

- [ ] **Step 1: Write failing tests**

```go
func TestMergeEnvs(t *testing.T) {
	base := &Env{Name: "shared", Vars: []*Var{
		{Key: "DB_HOST", Value: "localhost"},
		{Key: "SECRET_A", Value: "base-secret", Secret: true},
	}}
	override := &Env{Name: "prod", Vars: []*Var{
		{Key: "DB_HOST", Value: "prod-db.internal"},
		{Key: "EXTRA", Value: "1"},
	}}

	got := MergeEnvs(base, override)

	if got.Name != "prod" {
		t.Fatalf("Name = %q", got.Name)
	}
	m := got.Map()
	if m["DB_HOST"] != "prod-db.internal" {
		t.Fatalf("override must win: %v", m)
	}
	if m["SECRET_A"] != "base-secret" {
		t.Fatalf("base-only vars must survive: %v", m)
	}
	if m["EXTRA"] != "1" {
		t.Fatalf("override-only vars must appear: %v", m)
	}
	if len(got.Vars) != 3 {
		t.Fatalf("len = %d, want 3", len(got.Vars))
	}
	// inputs must not be mutated
	if len(base.Vars) != 2 || len(override.Vars) != 2 {
		t.Fatal("inputs were mutated")
	}
}
```

- [ ] **Step 2:** `go test ./internal/model/` — must fail.
- [ ] **Step 3:** Implement, rerun — must pass.

---

### Task 3: `(*Project).ResolveEnv` with chain walking + cycle detection

**Files:** `internal/model/resolve.go`, `internal/model/resolve_test.go`

```go
// maxResolveDepth guards against accidental deep chains.
const maxResolveDepth = 8

// ResolveEnv returns the env with the given name, fully resolved by folding
// its Base chain (deepest base first, own vars winning last).
// Returns an error if the env is missing, the chain is too deep, or there is a cycle.
func (p *Project) ResolveEnv(name string) (*Env, error) {
	e := p.FindEnv(name)
	if e == nil {
		return nil, fmt.Errorf("env %q not found in project %q", name, p.Name)
	}
	// Build chain from target back to the root base.
	chain := []*Env{}
	seen := map[string]bool{}
	for cur := e; cur != nil; {
		if len(chain) >= maxResolveDepth {
			return nil, fmt.Errorf("inheritance chain too deep at env %q", cur.Name)
		}
		if seen[cur.Name] {
			return nil, fmt.Errorf("inheritance cycle detected at env %q in project %q", cur.Name, p.Name)
		}
		seen[cur.Name] = true
		chain = append(chain, cur)
		if cur.Base == "" {
			break
		}
		cur = p.FindEnv(cur.Base)
		if cur == nil {
			return nil, fmt.Errorf("base env %q not found in project %q", chain[len(chain)-1].Base, p.Name)
		}
	}
	// Fold from deepest base up to the target.
	result := chain[len(chain)-1]
	for i := len(chain) - 2; i >= 0; i-- {
		result = MergeEnvs(result, chain[i])
	}
	return result, nil
}
```

- [ ] **Step 1: Write failing tests (table-driven)**

```go
func TestResolveEnv(t *testing.T) {
	newEnv := func(name, base string, vars ...*Var) *Env {
		return &Env{Name: name, Base: base, Vars: vars}
	}
	proj := &Project{Name: "api", Envs: []*Env{
		newEnv("shared", "",
			&Var{Key: "DB_HOST", Value: "localhost"},
			&Var{Key: "PORT", Value: "3000"}),
		newEnv("dev", "shared",
			&Var{Key: "DEBUG", Value: "true"}),
		newEnv("prod", "dev",
			&Var{Key: "DB_HOST", Value: "prod-db"},
			&Var{Key: "DEBUG", Value: "false"}),
	}}

	tests := []struct {
		name   string
		envName string
		want   map[string]string
	}{
		{"no base", "shared", map[string]string{"DB_HOST": "localhost", "PORT": "3000"}},
		{"one level", "dev", map[string]string{"DB_HOST": "localhost", "PORT": "3000", "DEBUG": "true"}},
		{"two levels", "prod", map[string]string{"DB_HOST": "prod-db", "PORT": "3000", "DEBUG": "false"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := proj.ResolveEnv(tt.envName)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Map(), tt.want) {
				t.Fatalf("got %v, want %v", got.Map(), tt.want)
			}
		})
	}
}

func TestResolveEnvErrors(t *testing.T) {
	newEnv := func(name, base string) *Env { return &Env{Name: name, Base: base} }

	tests := []struct {
		name     string
		envs     []*Env
		target   string
		wantErr  string
	}{
		{"missing env", []*Env{newEnv("a", "")}, "zzz", `env "zzz" not found`},
		{"missing base", []*Env{newEnv("a", "ghost")}, "a", `base env "ghost" not found`},
		{"self cycle", []*Env{newEnv("a", "a")}, "a", "cycle detected"},
		{"two-node cycle", []*Env{newEnv("a", "b"), newEnv("b", "a")}, "a", "cycle detected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Project{Name: "p", Envs: tt.envs}
			_, err := p.ResolveEnv(tt.target)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
```

(Add `reflect`, `strings`, `fmt` imports where needed. A long-chain >8 fixture test may also be added following the same pattern.)

- [ ] **Step 2:** `go test ./internal/model/` — must fail.
- [ ] **Step 3:** Implement, rerun — must pass.

---

### Task 4: `--resolve` flag on `get`, `env`, `export`

**Files:** `internal/cli/commands.go`, `internal/cli/cli_more_test.go`

Add a small helper to `internal/cli/commands.go`:

```go
// lookupEnv resolves a project/env pair, optionally folding the inheritance chain.
func lookupEnv(v *model.Vault, projectName, envName string, resolve bool) (*model.Env, error) {
	p := v.FindProject(projectName)
	if p == nil {
		return nil, fmt.Errorf("project %q not found", projectName)
	}
	if resolve {
		return p.ResolveEnv(envName)
	}
	e := p.FindEnv(envName)
	if e == nil {
		return nil, fmt.Errorf("env %q not found in project %q", envName, projectName)
	}
	return e, nil
}
```

In each of `getCmd`, `envCmd`, `exportCmd` (only those that exist in the codebase):
1. Add `var resolve bool` and `cmd.Flags().BoolVar(&resolve, "resolve", false, "resolve env inheritance (base chain) before use")`.
2. Replace the `resolveEnv(v, args[0], args[1])` call with `lookupEnv(v, args[0], args[1], resolve)`.

Leave `runCmd` untouched.

- [ ] **Step 1: Write failing tests**

Using the test helpers from `cli_more_test.go` (`withVaultPath`, `withIO`, `seedVault`, `fakeIO`), build a vault whose project "api" has envs `shared` (VAR=base) and `dev` (Base="shared", VAR=overridden, plus EXTRA=1), then:

```go
func TestExportResolve(t *testing.T) {
	// seed vault with shared/dev inheritance as described above
	// fakeIO with ENVII_PASSPHRASE
	// call exportCmd with flags set: parse flags first:
	cmd := exportCmd()
	if err := cmd.ParseFlags([]string{"--resolve"}); err != nil { t.Fatal(err) }
	if err := cmd.RunE(cmd, []string{"api", "dev"}); err != nil { t.Fatal(err) }
	// expect stdout to contain both EXTRA=1 and VAR=overridden
}
```

Follow exactly how existing command tests in `cli_more_test.go` parse flags and invoke RunE. Repeat an analogous test for `getCmd`/`envCmd` if those commands exist.

- [ ] **Step 2:** `go test ./internal/cli/` — must fail.
- [ ] **Step 3:** Implement, rerun — must pass.

---

### Task 5: Docs

- [ ] `README.md`: add an `### Environment inheritance` subsection showing:
  - how `base` is declared (via TUI-created envs edited... or note that `base` is set via vault JSON / future TUI support)
  - an example chain with `--resolve` on `envii env`/`envii export`.

### Task 6: Final verification

- [ ] `go build ./...`
- [ ] `go test ./...`
- [ ] `go vet ./...`

## Done When

- A three-level chain resolves with correct override precedence.
- Cycles and missing bases produce clear errors.
- `--resolve` off keeps current behavior byte-for-byte identical.
- Old vaults load unchanged.
