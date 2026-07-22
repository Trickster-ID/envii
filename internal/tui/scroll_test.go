package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/trickylab/envii/internal/model"
)

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
