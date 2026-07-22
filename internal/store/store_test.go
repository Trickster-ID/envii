package store

import (
	"path/filepath"
	"testing"

	"github.com/trickylab/envii/internal/crypto"
	"github.com/trickylab/envii/internal/model"
)

func init() { crypto.SetWorkFactor(10) }

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	s, err := New(path)
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	if s.Exists() {
		t.Fatal("expected vault to not exist yet")
	}

	v := model.NewVault()
	v.Projects = append(v.Projects, &model.Project{
		Name: "api",
		Envs: []*model.Env{{
			Name: "dev",
			Vars: []*model.Var{{Key: "PORT", Value: "8080"}},
		}},
	})

	if err := s.Save(v, "pass"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if !s.Exists() {
		t.Fatal("expected vault to exist after save")
	}

	got, err := s.Load("pass")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	p := got.FindProject("api")
	if p == nil {
		t.Fatal("project api not found")
	}
	if v := p.FindEnv("dev").Map()["PORT"]; v != "8080" {
		t.Fatalf("PORT = %q, want 8080", v)
	}
}

func TestLoadNotFound(t *testing.T) {
	s, err := New(filepath.Join(t.TempDir(), "missing.age"))
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := s.Load("x"); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
