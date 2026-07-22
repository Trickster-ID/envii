package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handle saveResult in Update via a small wrapper.
func (m Model) updateSave(msg saveResult) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errMsg = "save failed: " + msg.err.Error()
		return m, nil
	}
	m.dirty = false
	m.status = "vault saved"
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("envii"))
	b.WriteString("  ")
	b.WriteString(breadcrumbStyle.Render(m.breadcrumb()))
	if m.search != "" {
		b.WriteString("  ")
		b.WriteString(helpStyle.Render("filter: " + m.search))
	}
	b.WriteString("\n\n")

	b.WriteString(m.list())
	b.WriteString("\n")

	switch m.inputMode {
	case inputConfirmDelete:
		b.WriteString(errorStyle.Render("Delete " + m.deleteTarget + "?"))
		b.WriteString("\n")
		b.WriteString(inputBoxStyle.Render(m.input.View()))
		b.WriteString("\n")
	case inputSearch:
		b.WriteString(inputBoxStyle.Render("/ " + m.input.View()))
		b.WriteString("\n")
	case inputNone:
		// nothing
	default:
		b.WriteString(inputBoxStyle.Render(m.input.View()))
		b.WriteString("\n")
	}

	if m.errMsg != "" {
		b.WriteString(errorStyle.Render("✗ " + m.errMsg))
		b.WriteString("\n")
	} else if m.status != "" {
		b.WriteString(statusStyle.Render("✓ " + m.status))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render(m.help()))
	return b.String()
}

func (m Model) breadcrumb() string {
	parts := []string{"projects"}
	if m.level >= levelEnvs && len(m.vault.Projects) > 0 {
		parts = append(parts, m.vault.Projects[m.pIdx].Name)
	}
	if m.level >= levelVars {
		p := m.vault.Projects[m.pIdx]
		if len(p.Envs) > 0 {
			parts = append(parts, p.Envs[m.eIdx].Name)
		}
	}
	return strings.Join(parts, " / ")
}

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

func (m Model) row(selected bool, text string) string {
	if selected {
		return selectedStyle.Render("› " + text)
	}
	return normalStyle.Render("  " + text)
}

func (m Model) help() string {
	if m.level == levelVars {
		return "↑/↓ move • esc back • a add • i import • e edit • d del • / search • r reveal • c copy • s save • q quit"
	}
	return "↑/↓ move • enter open • esc back • a add • d delete • / search • s save • q quit"
}

func mask(s string) string {
	if len(s) == 0 {
		return "(empty)"
	}
	return strings.Repeat("•", min(len(s), 12))
}
