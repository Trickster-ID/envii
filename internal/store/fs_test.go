package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/trickylab/envii/internal/crypto"
	"github.com/trickylab/envii/internal/model"
	"github.com/trickylab/envii/internal/store"
)

func init() { crypto.SetWorkFactor(10) }

func TestStoreLoadAndSaveWithInjectedFS(t *testing.T) {
	tests := []struct {
		name string
		pass string
	}{
		{name: "basic", pass: "passphrase"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "vault.age")
			s, err := store.New(path, store.WithFS(store.OSFS{}))
			if err != nil {
				t.Fatalf("new: %v", err)
			}

			v := model.NewVault()
			v.Projects = append(v.Projects, &model.Project{Name: "api"})
			if err := s.Save(v, tt.pass); err != nil {
				t.Fatalf("save: %v", err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("expected file: %v", err)
			}

			got, err := s.Load(tt.pass)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if got.FindProject("api") == nil {
				t.Fatal("project api missing")
			}
		})
	}
}
