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
