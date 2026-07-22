// Package tui implements the interactive terminal UI for browsing and
// editing the vault.
package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/trickylab/envii/internal/model"
	"github.com/trickylab/envii/internal/runner"
	"github.com/trickylab/envii/internal/store"
)

// level represents the current navigation depth.
type level int

const (
	levelProjects level = iota
	levelEnvs
	levelVars
)

// inputMode is the active text-input action, if any.
type inputMode int

const (
	inputNone inputMode = iota
	inputAddProject
	inputAddEnv
	inputAddVarKey
	inputAddVarValue
	inputEditValue
	inputImportFile
	inputSearch
	inputConfirmDelete
)

// Model is the root Bubble Tea model.
type Model struct {
	vault      *model.Vault
	store      *store.Store
	passphrase string

	level level
	pIdx  int // selected project index
	eIdx  int // selected env index
	vIdx  int // selected var index

	reveal map[int]bool // var index -> revealed

	input      textinput.Model
	inputMode  inputMode
	pendingKey string // buffer for add-var key

	search       string // active search query
	deleteTarget string // label of item pending delete confirm

	width  int
	height int
	offset int // list viewport scroll offset (filtered-row index)

	status string
	errMsg string
	dirty  bool
}

// New constructs the TUI model.
func New(v *model.Vault, s *store.Store, passphrase string) Model {
	ti := textinput.New()
	ti.CharLimit = 4096
	ti.Width = 40

	return Model{
		vault:      v,
		store:      s,
		passphrase: passphrase,
		level:      levelProjects,
		reveal:     map[int]bool{},
		input:      ti,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		sel, total := m.filteredSelection()
		m.syncOffset(sel, total)
		return m, nil

	case tea.KeyMsg:
		if m.inputMode != inputNone {
			return m.updateInput(msg)
		}
		return m.updateNav(msg)

	case saveResult:
		return m.updateSave(msg)
	}
	return m, nil
}

func (m Model) updateNav(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.errMsg = ""
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)

	case "enter", "l", "right":
		m.descend()
	case "esc", "h", "left":
		m.ascend()

	case "a":
		m.startAdd()
	case "e":
		m.startEdit()
	case "d":
		m.startDeleteConfirm()
	case "i":
		m.startImport()
	case "/":
		m.startSearch()
	case "r":
		m.toggleReveal()
	case "c":
		m.copyValue()
	case "s":
		return m, m.save()
	}
	return m, nil
}

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

func (m *Model) descend() {
	switch m.level {
	case levelProjects:
		if len(m.vault.Projects) == 0 {
			return
		}
		m.level = levelEnvs
		m.eIdx = 0
		m.offset = 0
	case levelEnvs:
		if len(m.currentProject().Envs) == 0 {
			return
		}
		m.level = levelVars
		m.vIdx = 0
		m.reveal = map[int]bool{}
		m.offset = 0
	}
}

func (m *Model) ascend() {
	switch m.level {
	case levelVars:
		m.level = levelEnvs
		m.offset = 0
	case levelEnvs:
		m.level = levelProjects
		m.offset = 0
	}
}

func (m *Model) toggleReveal() {
	if m.level == levelVars && len(m.currentEnv().Vars) > 0 {
		m.reveal[m.vIdx] = !m.reveal[m.vIdx]
	}
}

func (m *Model) copyValue() {
	if m.level != levelVars || len(m.currentEnv().Vars) == 0 {
		return
	}
	v := m.currentEnv().Vars[m.vIdx]
	if err := clipboard.WriteAll(v.Value); err != nil {
		m.errMsg = "clipboard unavailable: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("copied %s to clipboard", v.Key)
}

func (m *Model) deleteCurrent() {
	switch m.level {
	case levelProjects:
		if len(m.vault.Projects) == 0 {
			return
		}
		m.vault.Projects = removeAt(m.vault.Projects, m.pIdx)
		m.pIdx = clamp(m.pIdx, len(m.vault.Projects))
	case levelEnvs:
		p := m.currentProject()
		if len(p.Envs) == 0 {
			return
		}
		p.Envs = removeAt(p.Envs, m.eIdx)
		m.eIdx = clamp(m.eIdx, len(p.Envs))
	case levelVars:
		e := m.currentEnv()
		if len(e.Vars) == 0 {
			return
		}
		e.Vars = removeAt(e.Vars, m.vIdx)
		m.vIdx = clamp(m.vIdx, len(e.Vars))
	}
	m.dirty = true
	m.status = "deleted (press s to save)"
	sel, total := m.filteredSelection()
	m.syncOffset(sel, total)
}

func (m Model) save() tea.Cmd {
	return func() tea.Msg {
		if err := m.store.Save(m.vault, m.passphrase); err != nil {
			return saveResult{err: err}
		}
		return saveResult{}
	}
}

type saveResult struct{ err error }

// --- input handling ---

func (m *Model) startDeleteConfirm() {
	var label string
	switch m.level {
	case levelProjects:
		if len(m.vault.Projects) == 0 {
			return
		}
		label = fmt.Sprintf("project %q", m.vault.Projects[m.pIdx].Name)
	case levelEnvs:
		p := m.currentProject()
		if len(p.Envs) == 0 {
			return
		}
		label = fmt.Sprintf("env %q", p.Envs[m.eIdx].Name)
	case levelVars:
		e := m.currentEnv()
		if len(e.Vars) == 0 {
			return
		}
		label = fmt.Sprintf("var %q", e.Vars[m.vIdx].Key)
	}
	m.deleteTarget = label
	m.inputMode = inputConfirmDelete
	m.input.SetValue("")
	m.input.Placeholder = "y to confirm"
	m.input.Focus()
}

func (m *Model) startSearch() {
	m.inputMode = inputSearch
	m.input.SetValue(m.search)
	m.input.Placeholder = "search..."
	m.input.Focus()
}

func (m *Model) startAdd() {
	m.input.SetValue("")
	m.input.Focus()
	switch m.level {
	case levelProjects:
		m.inputMode = inputAddProject
		m.input.Placeholder = "project name"
	case levelEnvs:
		m.inputMode = inputAddEnv
		m.input.Placeholder = "env name (e.g. dev)"
	case levelVars:
		m.inputMode = inputAddVarKey
		m.input.Placeholder = "KEY"
	}
}

func (m *Model) startEdit() {
	if m.level != levelVars || len(m.currentEnv().Vars) == 0 {
		return
	}
	m.inputMode = inputEditValue
	m.input.SetValue(m.currentEnv().Vars[m.vIdx].Value)
	m.input.Placeholder = "value"
	m.input.Focus()
}

func (m *Model) startImport() {
	if m.level != levelVars {
		return
	}
	m.inputMode = inputImportFile
	m.input.SetValue("")
	m.input.Placeholder = ".env file path"
	m.input.Focus()
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.inputMode == inputSearch {
			m.search = "" // clear filter on esc
			m.offset = 0
		}
		m.inputMode = inputNone
		m.input.Blur()
		return m, nil
	case "enter":
		if m.inputMode == inputSearch {
			// Commit search query and close input box; filter stays active.
			m.search = m.input.Value()
			m.inputMode = inputNone
			m.input.Blur()
			return m, nil
		}
		return m.commitInput()
	}
	// For search: update filter live as user types.
	if m.inputMode == inputSearch {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		m.search = m.input.Value()
		sel, total := m.filteredSelection()
		m.syncOffset(sel, total)
		return m, cmd
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) commitInput() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(m.input.Value())
	switch m.inputMode {
	case inputConfirmDelete:
		m.inputMode = inputNone
		m.input.Blur()
		m.deleteTarget = ""
		if strings.ToLower(val) == "y" {
			m.deleteCurrent()
		} else {
			m.status = "delete cancelled"
		}
		return m, nil
	case inputAddProject:
		if val != "" {
			m.vault.Projects = append(m.vault.Projects, &model.Project{Name: val})
			m.dirty = true
		}
	case inputAddEnv:
		if val != "" {
			p := m.currentProject()
			p.Envs = append(p.Envs, &model.Env{Name: val})
			m.dirty = true
		}
	case inputAddVarKey:
		if val != "" {
			m.pendingKey = val
			m.input.SetValue("")
			m.input.Placeholder = "value"
			m.inputMode = inputAddVarValue
			return m, nil
		}
	case inputAddVarValue:
		e := m.currentEnv()
		e.Vars = append(e.Vars, &model.Var{Key: m.pendingKey, Value: val, Secret: model.IsSecret(m.pendingKey)})
		m.pendingKey = ""
		m.dirty = true
	case inputEditValue:
		m.currentEnv().Vars[m.vIdx].Value = val
		m.dirty = true
	case inputImportFile:
		m.inputMode = inputNone
		m.input.Blur()
		if val == "" {
			m.errMsg = "import file path cannot be empty"
			return m, nil
		}
		if err := m.importFile(val); err != nil {
			m.errMsg = err.Error()
		}
		return m, nil
	}
	m.inputMode = inputNone
	m.input.Blur()
	m.status = "changed (press s to save)"
	return m, nil
}

// --- helpers ---

func (m *Model) currentProject() *model.Project {
	if len(m.vault.Projects) == 0 {
		return &model.Project{}
	}
	return m.vault.Projects[m.pIdx]
}

func (m *Model) currentEnv() *model.Env {
	p := m.currentProject()
	if len(p.Envs) == 0 {
		return &model.Env{}
	}
	return p.Envs[m.eIdx]
}

func clamp(i, n int) int {
	if n == 0 {
		return 0
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

func removeAt[T any](s []T, i int) []T {
	return append(s[:i], s[i+1:]...)
}


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

func (m *Model) importFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	vars, err := runner.ParseDotenv(f)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if len(vars) == 0 {
		return fmt.Errorf("%s contains no variables", path)
	}

	summary := model.ImportVars(m.currentEnv(), vars, false)
	if summary.Added > 0 {
		m.dirty = true
		m.status = fmt.Sprintf("imported %d vars (%d skipped, press s to save)", summary.Added, summary.Skipped)
		return nil
	}
	m.status = fmt.Sprintf("imported 0 vars (%d skipped)", summary.Skipped)
	return nil
}
