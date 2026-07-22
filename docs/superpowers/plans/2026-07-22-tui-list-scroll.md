# TUI List Scroll Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep the selected TUI list row on-screen when the item count exceeds terminal height by adding a viewport offset to the custom Bubble Tea list.

**Architecture:** Keep the existing thin custom list (no `bubbles/list` / `bubbles/viewport`). Add one `offset` on `Model`, compute list window height as `height - chrome`, clamp `offset` so the selected filtered row stays in `[offset, offset+visible)`, and slice rows in `list()`. Reset/reclamp offset on level change, delete, search, and resize.

**Tech Stack:** Go 1.26, Bubble Tea (`charmbracelet/bubbletea`), `go test` table-driven tests, no new deps.

## Global Constraints

- Package: `internal/tui` only
- No new dependencies
- No `bubbles/list` / `bubbles/viewport`
- Single shared `offset int` (not per-level) — reset on level change / search clear
- Chrome (title/breadcrumb/status/help/input) must never be covered by the list
- Search filter + empty state behavior stays correct
- Table-driven tests required for scroll helpers and list windowing
- No commits without explicit user approval in execution session

---

## File Structure

### Create
- `internal/tui/scroll.go` — pure helpers: chrome height, visible count, ensure-cursor-visible
- `internal/tui/scroll_test.go` — table tests for helpers
- `internal/tui/view_test.go` — list windowing / View line-count tests

### Modify
- `internal/tui/tui.go` — add `offset`, call ensure on move/level/delete/resize
- `internal/tui/view.go` — slice filtered rows by `offset` + `visible`

---

## Tasks

### Task 1: Scroll helpers (pure functions)

**Files:**
- Create: `internal/tui/scroll.go`
- Test: `internal/tui/scroll_test.go`

**Interfaces:**
- Consumes: nothing (stdlib only)
- Produces:
  - `func chromeLines(inputActive bool, hasStatus bool) int`
  - `func listVisible(height, chrome int) int`
  - `func ensureOffset(offset, cursor, visible, total int) int`

- [ ] **Step 1: Write failing tests**

```go
package tui

import "testing"

func TestChromeLines(t *testing.T) {
	tests := []struct {
		name        string
		inputActive bool
		hasStatus   bool
		want        int
	}{
		// base: title+blank + blank-after-list + help = 4
		{"base", false, false, 4},
		{"status", false, true, 5},
		{"input", true, false, 6}, // input box ~2 lines
		{"both", true, true, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chromeLines(tt.inputActive, tt.hasStatus); got != tt.want {
				t.Fatalf("chromeLines(%v,%v)=%d want %d", tt.inputActive, tt.hasStatus, got, tt.want)
			}
		})
	}
}

func TestListVisible(t *testing.T) {
	tests := []struct {
		name           string
		height, chrome int
		want           int
	}{
		{"normal", 24, 4, 20},
		{"tiny", 5, 4, 1},
		{"zero height", 0, 4, 1},
		{"chrome bigger", 3, 10, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := listVisible(tt.height, tt.chrome); got != tt.want {
				t.Fatalf("listVisible(%d,%d)=%d want %d", tt.height, tt.chrome, got, tt.want)
			}
		})
	}
}

func TestEnsureOffset(t *testing.T) {
	tests := []struct {
		name                           string
		offset, cursor, visible, total int
		want                           int
	}{
		{"no scroll needed", 0, 2, 10, 5, 0},
		{"cursor below window", 0, 12, 10, 20, 3}, // 12 must be in [3,13)
		{"cursor above window", 5, 2, 10, 20, 2},
		{"empty list", 0, 0, 10, 0, 0},
		{"visible covers all", 3, 1, 20, 5, 0},
		{"cursor last item", 0, 19, 10, 20, 10},
		{"offset past end clamped", 50, 5, 10, 20, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ensureOffset(tt.offset, tt.cursor, tt.visible, tt.total)
			if got != tt.want {
				t.Fatalf("ensureOffset(%d,%d,%d,%d)=%d want %d",
					tt.offset, tt.cursor, tt.visible, tt.total, got, tt.want)
			}
			if tt.total > 0 && tt.visible > 0 && tt.cursor >= 0 && tt.cursor < tt.total {
				if tt.cursor < got || tt.cursor >= got+tt.visible {
					t.Fatalf("cursor %d not in [%d,%d)", tt.cursor, got, got+tt.visible)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

Run:
```sh
go test ./internal/tui/ -run 'TestChromeLines|TestListVisible|TestEnsureOffset' -v
```

Expected: FAIL — undefined `chromeLines`, `listVisible`, `ensureOffset`.

- [ ] **Step 3: Minimal implementation**

```go
package tui

// chromeLines returns non-list lines consumed by View chrome.
// base: title line, blank after title, blank after list, help line.
// +2 when an input box is shown; +1 when status or error is shown.
func chromeLines(inputActive, hasStatus bool) int {
	n := 4
	if inputActive {
		n += 2
	}
	if hasStatus {
		n++
	}
	return n
}

// listVisible returns how many list rows fit. Always ≥ 1.
func listVisible(height, chrome int) int {
	v := height - chrome
	if v < 1 {
		return 1
	}
	return v
}

// ensureOffset returns a new scroll offset so cursor is inside the window
// [offset, offset+visible). Also clamps offset into [0, max(0, total-visible)].
func ensureOffset(offset, cursor, visible, total int) int {
	if total <= 0 || visible <= 0 {
		return 0
	}
	if visible >= total {
		return 0
	}
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+visible {
		offset = cursor - visible + 1
	}
	maxOff := total - visible
	if offset < 0 {
		offset = 0
	}
	if offset > maxOff {
		offset = maxOff
	}
	return offset
}
```

- [ ] **Step 4: Run tests — expect PASS**

Run:
```sh
go test ./internal/tui/ -run 'TestChromeLines|TestListVisible|TestEnsureOffset' -v
```

Expected: PASS.

- [ ] **Step 5: Commit** (only if user approves)

```sh
git add internal/tui/scroll.go internal/tui/scroll_test.go
git commit -m "feat(tui): add list viewport scroll helpers"
```

---

### Task 2: Wire offset into Model + moveCursor / level / resize

**Files:**
- Modify: `internal/tui/tui.go`
- Test: `internal/tui/scroll_test.go` (append model-level tests)

**Interfaces:**
- Consumes: `ensureOffset`, `listVisible`, `chromeLines` from Task 1
- Produces:
  - `Model.offset int`
  - `func (m Model) visibleRows() int`
  - `func (m *Model) syncOffset(selectedInFiltered, filteredTotal int)`
  - `func (m Model) filteredSelection() (sel, total int)`
  - `moveCursor` / `descend` / `ascend` / `deleteCurrent` / `WindowSizeMsg` / search update offset

- [ ] **Step 1: Write failing model tests**

Append to `internal/tui/scroll_test.go` (add imports `"fmt"`, `tea "github.com/charmbracelet/bubbletea"`, `"github.com/trickylab/envii/internal/model"`):

```go
func TestMoveCursorScrollsOffset(t *testing.T) {
	v := &model.Vault{Projects: make([]*model.Project, 30)}
	for i := range v.Projects {
		v.Projects[i] = &model.Project{Name: fmt.Sprintf("p%02d", i)}
	}
	m := New(v, nil, "")
	m.height = 14
	m.width = 80

	for i := 0; i < 12; i++ {
		m.moveCursor(1)
	}
	if m.pIdx != 12 {
		t.Fatalf("pIdx=%d want 12", m.pIdx)
	}
	vis := m.visibleRows()
	if m.pIdx < m.offset || m.pIdx >= m.offset+vis {
		t.Fatalf("cursor %d outside window [%d,%d)", m.pIdx, m.offset, m.offset+vis)
	}
	if m.offset == 0 {
		t.Fatal("offset still 0 after moving past viewport")
	}

	for m.pIdx > 0 {
		m.moveCursor(-1)
	}
	if m.offset != 0 {
		t.Fatalf("offset=%d want 0 after cursor at top", m.offset)
	}
}

func TestDescendResetsOffset(t *testing.T) {
	envs := make([]*model.Env, 25)
	for i := range envs {
		envs[i] = &model.Env{Name: fmt.Sprintf("e%02d", i)}
	}
	v := &model.Vault{Projects: []*model.Project{{Name: "app", Envs: envs}}}
	m := New(v, nil, "")
	m.height = 14
	m.offset = 7
	m.descend()
	if m.level != levelEnvs {
		t.Fatalf("level=%v want envs", m.level)
	}
	if m.offset != 0 {
		t.Fatalf("offset=%d want 0 after descend", m.offset)
	}
}

func TestWindowResizeKeepsCursorVisible(t *testing.T) {
	v := &model.Vault{Projects: make([]*model.Project, 40)}
	for i := range v.Projects {
		v.Projects[i] = &model.Project{Name: fmt.Sprintf("p%02d", i)}
	}
	m := New(v, nil, "")
	m.height = 30
	m.pIdx = 25
	m.offset = 20
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 12})
	m = updated.(Model)
	vis := m.visibleRows()
	if m.pIdx < m.offset || m.pIdx >= m.offset+vis {
		t.Fatalf("after resize cursor %d outside [%d,%d)", m.pIdx, m.offset, m.offset+vis)
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

Run:
```sh
go test ./internal/tui/ -run 'TestMoveCursorScrollsOffset|TestDescendResetsOffset|TestWindowResizeKeepsCursorVisible' -v
```

Expected: FAIL — no `offset` field / `visibleRows` / offset not updated.

- [ ] **Step 3: Implement wiring in `tui.go`**

Add field on `Model` next to `height`:

```go
	width  int
	height int
	offset int // list viewport scroll offset (filtered-row index)
```

Add helpers (bottom of file, near other helpers):

```go
func (m Model) visibleRows() int {
	inputActive := m.inputMode != inputNone
	hasStatus := m.errMsg != "" || m.status != ""
	return listVisible(m.height, chromeLines(inputActive, hasStatus))
}

// syncOffset keeps the selected filtered-row index on screen.
// selectedInFiltered is index within filtered list; use -1 if none selected/matched.
func (m *Model) syncOffset(selectedInFiltered, filteredTotal int) {
	vis := m.visibleRows()
	cur := selectedInFiltered
	if cur < 0 {
		cur = 0
	}
	m.offset = ensureOffset(m.offset, cur, vis, filteredTotal)
}

// filteredSelection returns (index among items matching search, filtered count).
func (m Model) filteredSelection() (sel, total int) {
	q := strings.ToLower(m.search)
	sel = -1
	switch m.level {
	case levelProjects:
		for i, p := range m.vault.Projects {
			if q != "" && !strings.Contains(strings.ToLower(p.Name), q) {
				continue
			}
			if i == m.pIdx {
				sel = total
			}
			total++
		}
	case levelEnvs:
		for i, e := range m.currentProject().Envs {
			if q != "" && !strings.Contains(strings.ToLower(e.Name), q) {
				continue
			}
			if i == m.eIdx {
				sel = total
			}
			total++
		}
	case levelVars:
		for i, v := range m.currentEnv().Vars {
			if q != "" && !strings.Contains(strings.ToLower(v.Key), q) && !strings.Contains(strings.ToLower(v.Value), q) {
				continue
			}
			if i == m.vIdx {
				sel = total
			}
			total++
		}
	}
	return sel, total
}
```

Replace `moveCursor`:

```go
func (m *Model) moveCursor(delta int) {
	switch m.level {
	case levelProjects:
		m.pIdx = clamp(m.pIdx+delta, len(m.vault.Projects))
	case levelEnvs:
		m.eIdx = clamp(m.eIdx+delta, len(m.currentProject().Envs))
	case levelVars:
		m.vIdx = clamp(m.vIdx+delta, len(m.currentEnv().Vars))
	}
	sel, total := m.filteredSelection()
	m.syncOffset(sel, total)
}
```

Reset offset on level change — in `descend` set `m.offset = 0` after each successful level change; same in `ascend`.

After `deleteCurrent` body (after status assign):

```go
	sel, total := m.filteredSelection()
	m.syncOffset(sel, total)
```

`WindowSizeMsg` branch:

```go
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		sel, total := m.filteredSelection()
		m.syncOffset(sel, total)
		return m, nil
```

Search esc clear in `updateInput`:

```go
	case "esc":
		if m.inputMode == inputSearch {
			m.search = ""
			m.offset = 0
		}
		m.inputMode = inputNone
		m.input.Blur()
		return m, nil
```

Live search typing branch — after `m.search = m.input.Value()`:

```go
		sel, total := m.filteredSelection()
		m.syncOffset(sel, total)
```

- [ ] **Step 4: Run tests — expect PASS**

Run:
```sh
go test ./internal/tui/ -run 'TestMoveCursorScrollsOffset|TestDescendResetsOffset|TestWindowResizeKeepsCursorVisible' -v
```

Expected: PASS.

- [ ] **Step 5: Commit** (only if user approves)

```sh
git add internal/tui/tui.go internal/tui/scroll_test.go
git commit -m "feat(tui): track list scroll offset on move/level/resize"
```

---

### Task 3: Slice list rows by viewport in `list()`

**Files:**
- Modify: `internal/tui/view.go`
- Test: `internal/tui/view_test.go`

**Interfaces:**
- Consumes: `Model.offset`, `visibleRows`, `ensureOffset`
- Produces: `list()` returns at most `visibleRows()` lines; selected row present when item exists

- [ ] **Step 1: Write failing view tests**

Create `internal/tui/view_test.go`:

```go
package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/trickylab/envii/internal/model"
)

func TestListWindowsToVisibleHeight(t *testing.T) {
	v := &model.Vault{Projects: make([]*model.Project, 50)}
	for i := range v.Projects {
		v.Projects[i] = &model.Project{Name: fmt.Sprintf("project-%02d", i)}
	}
	m := New(v, nil, "")
	m.height = 12
	m.width = 80
	m.pIdx = 0
	m.offset = 0

	out := m.list()
	lines := strings.Split(out, "\n")
	vis := m.visibleRows()
	if len(lines) > vis {
		t.Fatalf("list lines=%d > visible=%d\n%s", len(lines), vis, out)
	}
	if !strings.Contains(out, "›") {
		t.Fatal("selected marker missing at top")
	}
}

func TestListScrollShowsSelectedNearBottom(t *testing.T) {
	v := &model.Vault{Projects: make([]*model.Project, 50)}
	for i := range v.Projects {
		v.Projects[i] = &model.Project{Name: fmt.Sprintf("project-%02d", i)}
	}
	m := New(v, nil, "")
	m.height = 12
	m.pIdx = 30
	m.syncOffset(30, 50)

	out := m.list()
	if !strings.Contains(out, "project-30") {
		t.Fatalf("selected project-30 not in viewport:\n%s", out)
	}
	if !strings.Contains(out, "›") {
		t.Fatal("selected marker missing")
	}
	if strings.Contains(out, "project-00") {
		t.Fatal("project-00 still visible — offset not applied")
	}
}

func TestListEmptyAndSearchUnchanged(t *testing.T) {
	m := New(&model.Vault{Projects: nil}, nil, "")
	m.height = 20
	out := m.list()
	if !strings.Contains(out, "no projects") {
		t.Fatalf("empty state broken: %q", out)
	}

	m.vault.Projects = []*model.Project{{Name: "alpha"}, {Name: "beta"}}
	m.search = "zz"
	out = m.list()
	if !strings.Contains(out, "no results") {
		t.Fatalf("search empty state broken: %q", out)
	}
}

func TestViewDoesNotExceedHeight(t *testing.T) {
	v := &model.Vault{Projects: make([]*model.Project, 80)}
	for i := range v.Projects {
		v.Projects[i] = &model.Project{Name: fmt.Sprintf("p%02d", i)}
	}
	m := New(v, nil, "")
	m.height = 15
	m.width = 80
	m.pIdx = 40
	m.syncOffset(40, 80)

	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) > m.height+1 {
		t.Fatalf("View lines=%d height=%d\n%s", len(lines), m.height, view)
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

Run:
```sh
go test ./internal/tui/ -run 'TestListWindows|TestListScroll|TestListEmpty|TestViewDoesNot' -v
```

Expected: FAIL — full list still joined; `project-00` still present when scrolled.

- [ ] **Step 3: Implement windowing in `list()`**

Replace body of `list()` in `view.go`. Keep empty-state early returns. Build full filtered `rows`, track `selectedInFiltered`, then slice with defensive `ensureOffset` for render only (do **not** mutate `m.offset` in View — Update already synced it).

```go
func (m Model) list() string {
	q := strings.ToLower(m.search)
	var rows []string
	selectedInFiltered := -1

	switch m.level {
	case levelProjects:
		if len(m.vault.Projects) == 0 {
			return normalStyle.Render("  no projects yet — press 'a' to add one")
		}
		for i, p := range m.vault.Projects {
			if q != "" && !strings.Contains(strings.ToLower(p.Name), q) {
				continue
			}
			if i == m.pIdx {
				selectedInFiltered = len(rows)
			}
			rows = append(rows, m.row(i == m.pIdx, fmt.Sprintf("%s (%d envs)", p.Name, len(p.Envs))))
		}
	case levelEnvs:
		envs := m.currentProject().Envs
		if len(envs) == 0 {
			return normalStyle.Render("  no environments — press 'a' to add one")
		}
		for i, e := range envs {
			if q != "" && !strings.Contains(strings.ToLower(e.Name), q) {
				continue
			}
			if i == m.eIdx {
				selectedInFiltered = len(rows)
			}
			rows = append(rows, m.row(i == m.eIdx, fmt.Sprintf("%s (%d vars)", e.Name, len(e.Vars))))
		}
	case levelVars:
		vars := m.currentEnv().Vars
		if len(vars) == 0 {
			return normalStyle.Render("  no variables — press 'a' to add one")
		}
		for i, v := range vars {
			if q != "" && !strings.Contains(strings.ToLower(v.Key), q) && !strings.Contains(strings.ToLower(v.Value), q) {
				continue
			}
			if i == m.vIdx {
				selectedInFiltered = len(rows)
			}
			val := v.Value
			if v.Secret && !m.reveal[i] {
				val = secretStyle.Render(mask(v.Value))
			}
			rows = append(rows, m.row(i == m.vIdx, fmt.Sprintf("%-24s %s", v.Key, val)))
		}
	}

	if len(rows) == 0 && q != "" {
		return normalStyle.Render(fmt.Sprintf("  no results for %q", q))
	}

	vis := m.visibleRows()
	cur := selectedInFiltered
	if cur < 0 {
		cur = 0
	}
	off := ensureOffset(m.offset, cur, vis, len(rows))
	end := off + vis
	if end > len(rows) {
		end = len(rows)
	}
	return strings.Join(rows[off:end], "\n")
}
```

- [ ] **Step 4: Run all tui tests — expect PASS**

Run:
```sh
go test ./internal/tui/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit** (only if user approves)

```sh
git add internal/tui/view.go internal/tui/tui.go internal/tui/view_test.go internal/tui/scroll_test.go
git commit -m "feat(tui): window list rows so cursor stays visible"
```

---

### Task 4: Manual acceptance + package check

**Files:** none (verification only)

- [ ] **Step 1: Unit suite green**

Run:
```sh
go test ./internal/tui/ -count=1
go vet ./internal/tui/
```

Expected: PASS, no vet issues.

- [ ] **Step 2: Manual TUI check**

1. Seed vault with 50+ vars (`docs/seed.sh` if present).
2. Shrink terminal ~15 rows.
3. Open TUI → `j`/`k` through projects/envs/vars.
4. Confirm: `›` always visible; list scrolls; help/status not covered.
5. Resize mid-nav — cursor stays on-screen.
6. `/` filter + empty state; esc clears filter.
7. `a` add / `d` delete near bottom — no panic; cursor valid.

Acceptance (issue #5):
- [ ] Short terminal + long list: cursor always visible on up/down
- [ ] Resize recomputes viewport; cursor on-screen
- [ ] Chrome (help/status/input) not covered by list
- [ ] Search filter + empty state still correct

- [ ] **Step 3: Final status**

```sh
git status
```

---

## Self-Review

**1. Spec coverage (issue #5):**
| Requirement | Task |
|---|---|
| Viewport follows cursor | 2 + 3 |
| Partial-window scroll (not full page jump) | `ensureOffset` edge-follow |
| `height` used for list windowing | `listVisible` + chrome |
| Header/status/input chrome preserved | `chromeLines` |
| All levels (projects/envs/vars) | shared `list()` + `filteredSelection` |
| Resize keeps cursor on-screen | `WindowSizeMsg` → `syncOffset` |
| Search + empty state | Task 3 tests + early returns |
| No bubbles/list | pure custom offset |

**2. Placeholder scan:** none.

**3. Type consistency:** `offset`, `chromeLines`, `listVisible`, `ensureOffset`, `visibleRows`, `syncOffset`, `filteredSelection` stable across tasks.

**Skipped (YAGNI):**
- Per-level offsets — reset on navigate enough; add if users lose project scroll after descend.
- `… N more above/below` — add when discoverability needed.
- `bubbles/viewport` — overkill.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-22-tui-list-scroll.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** — fresh subagent per task, review between tasks

**2. Inline Execution** — this session, checkpoints

Which approach?
