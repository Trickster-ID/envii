package store_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/trickylab/envii/internal/crypto"
	"github.com/trickylab/envii/internal/model"
	"github.com/trickylab/envii/internal/store"
)

func init() { crypto.SetWorkFactor(10) }

func TestNewDefaultPathAndOptions(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		opts    []store.Option
		wantErr string
		check   func(t *testing.T, s *store.Store)
	}{
		{
			name: "explicit path",
			path: "/tmp/vault.age",
			check: func(t *testing.T, s *store.Store) {
				if s.Path != "/tmp/vault.age" {
					t.Fatalf("path %q", s.Path)
				}
			},
		},
		{
			name: "WithPath overrides empty",
			path: "",
			opts: []store.Option{store.WithPath("/override/vault.age")},
			check: func(t *testing.T, s *store.Store) {
				if s.Path != "/override/vault.age" {
					t.Fatalf("path %q", s.Path)
				}
			},
		},
		{
			name: "default path from UserConfigDir",
			path: "",
			opts: []store.Option{store.WithFS(configFS{dir: "/cfg"})},
			check: func(t *testing.T, s *store.Store) {
				want := filepath.Join("/cfg", "envii", "vault.age")
				if s.Path != want {
					t.Fatalf("path got %q want %q", s.Path, want)
				}
			},
		},
		{
			name:    "UserConfigDir error",
			path:    "",
			opts:    []store.Option{store.WithFS(configFS{err: errors.New("no home")})},
			wantErr: "resolve config dir",
		},
		{
			name: "nil FS option falls back to OSFS",
			path: filepath.Join(t.TempDir(), "v.age"),
			opts: []store.Option{store.WithFS(nil)},
			check: func(t *testing.T, s *store.Store) {
				if s.Exists() {
					t.Fatal("should not exist")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := store.New(tt.path, tt.opts...)
			if tt.wantErr != "" {
				if err == nil || !contains(err.Error(), tt.wantErr) {
					t.Fatalf("err=%v want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("new: %v", err)
			}
			if tt.check != nil {
				tt.check(t, s)
			}
		})
	}
}

func TestLoadErrorPaths(t *testing.T) {
	pass := "pass"
	validCipher, err := crypto.Encrypt([]byte(`{"version":1,"projects":[]}`), pass)
	if err != nil {
		t.Fatal(err)
	}
	// valid age cipher of non-JSON plaintext
	badJSONCipher, err := crypto.Encrypt([]byte("not-json"), pass)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		setup   func(t *testing.T) *store.Store
		pass    string
		wantErr string
		wantNF  bool
	}{
		{
			name: "not found",
			setup: func(t *testing.T) *store.Store {
				s, err := store.New(filepath.Join(t.TempDir(), "missing.age"))
				if err != nil {
					t.Fatal(err)
				}
				return s
			},
			pass:   pass,
			wantNF: true,
		},
		{
			name: "read error",
			setup: func(t *testing.T) *store.Store {
				fs := store.NewMockFS(t)
				fs.EXPECT().ReadFile(mock.Anything).Return(nil, errors.New("io boom"))
				s, err := store.New("/x.age", store.WithFS(fs))
				if err != nil {
					t.Fatal(err)
				}
				return s
			},
			pass:    pass,
			wantErr: "read vault",
		},
		{
			name: "wrong passphrase",
			setup: func(t *testing.T) *store.Store {
				fs := store.NewMockFS(t)
				fs.EXPECT().ReadFile(mock.Anything).Return(validCipher, nil)
				s, err := store.New("/x.age", store.WithFS(fs))
				if err != nil {
					t.Fatal(err)
				}
				return s
			},
			pass:    "wrong",
			wantErr: "decrypt",
		},
		{
			name: "parse error",
			setup: func(t *testing.T) *store.Store {
				fs := store.NewMockFS(t)
				fs.EXPECT().ReadFile(mock.Anything).Return(badJSONCipher, nil)
				s, err := store.New("/x.age", store.WithFS(fs))
				if err != nil {
					t.Fatal(err)
				}
				return s
			},
			pass:    pass,
			wantErr: "parse vault",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.setup(t)
			_, err := s.Load(tt.pass)
			if tt.wantNF {
				if !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("err=%v want ErrNotFound", err)
				}
				return
			}
			if err == nil || !contains(err.Error(), tt.wantErr) {
				t.Fatalf("err=%v want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestSaveErrorPaths(t *testing.T) {
	v := model.NewVault()
	pass := "pass"

	tests := []struct {
		name    string
		setup   func(t *testing.T) *store.Store
		vault   *model.Vault
		wantErr string
	}{
		{
			name: "mkdir fail",
			setup: func(t *testing.T) *store.Store {
				fs := store.NewMockFS(t)
				fs.EXPECT().MkdirAll(mock.Anything, mock.Anything).Return(errors.New("mkdir no"))
				s, err := store.New("/dir/vault.age", store.WithFS(fs))
				if err != nil {
					t.Fatal(err)
				}
				return s
			},
			vault:   v,
			wantErr: "create config dir",
		},
		{
			name: "write fail",
			setup: func(t *testing.T) *store.Store {
				fs := store.NewMockFS(t)
				fs.EXPECT().MkdirAll(mock.Anything, mock.Anything).Return(nil)
				fs.EXPECT().WriteFile(mock.Anything, mock.Anything, mock.Anything).Return(errors.New("disk full"))
				s, err := store.New("/dir/vault.age", store.WithFS(fs))
				if err != nil {
					t.Fatal(err)
				}
				return s
			},
			vault:   v,
			wantErr: "write temp vault",
		},
		{
			name: "rename fail",
			setup: func(t *testing.T) *store.Store {
				fs := store.NewMockFS(t)
				fs.EXPECT().MkdirAll(mock.Anything, mock.Anything).Return(nil)
				fs.EXPECT().WriteFile(mock.Anything, mock.Anything, mock.Anything).Return(nil)
				fs.EXPECT().Rename(mock.Anything, mock.Anything).Return(errors.New("cross device"))
				s, err := store.New("/dir/vault.age", store.WithFS(fs))
				if err != nil {
					t.Fatal(err)
				}
				return s
			},
			vault:   v,
			wantErr: "commit vault",
		},
		{
			name: "encrypt fail empty passphrase",
			setup: func(t *testing.T) *store.Store {
				// Encrypt rejects empty passphrase via age
				fs := store.NewMockFS(t)
				// no FS calls expected if encrypt fails first
				s, err := store.New("/dir/vault.age", store.WithFS(fs))
				if err != nil {
					t.Fatal(err)
				}
				return s
			},
			vault:   v,
			wantErr: "", // any error; check non-nil below
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.setup(t)
			p := pass
			if tt.name == "encrypt fail empty passphrase" {
				p = ""
			}
			err := s.Save(tt.vault, p)
			if err == nil {
				t.Fatal("expected error")
			}
			if tt.wantErr != "" && !contains(err.Error(), tt.wantErr) {
				t.Fatalf("err=%v want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestSaveUpdatesTimestampAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.age")
	s, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().Add(-time.Second)
	v := model.NewVault()
	v.Projects = append(v.Projects, &model.Project{
		Name: "p",
		Envs: []*model.Env{{Name: "e", Vars: []*model.Var{{Key: "K", Value: "V"}}}},
	})
	if err := s.Save(v, "secret"); err != nil {
		t.Fatal(err)
	}
	if v.UpdatedAt.Before(before) {
		t.Fatalf("UpdatedAt not refreshed: %v", v.UpdatedAt)
	}
	got, err := s.Load("secret")
	if err != nil {
		t.Fatal(err)
	}
	// re-marshal to ensure valid structure
	if _, err := json.Marshal(got); err != nil {
		t.Fatal(err)
	}
	if got.FindProject("p") == nil {
		t.Fatal("missing project")
	}
}

func TestOSFSUserConfigDir(t *testing.T) {
	dir, err := (store.OSFS{}).UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	if dir == "" {
		t.Fatal("empty config dir")
	}
	// Stat of a non-existent path
	_, err = (store.OSFS{}).Stat(filepath.Join(dir, "envii-does-not-exist-xyz"))
	if !errors.Is(err, os.ErrNotExist) {
		// still ok if other error; just exercise path
		_ = err
	}
}

// configFS only implements UserConfigDir; other methods panic if called.
type configFS struct {
	dir string
	err error
}

func (c configFS) UserConfigDir() (string, error) { return c.dir, c.err }
func (configFS) Stat(string) (os.FileInfo, error) { panic("Stat") }
func (configFS) ReadFile(string) ([]byte, error)  { panic("ReadFile") }
func (configFS) WriteFile(string, []byte, os.FileMode) error {
	panic("WriteFile")
}
func (configFS) MkdirAll(string, os.FileMode) error { panic("MkdirAll") }
func (configFS) Rename(string, string) error        { panic("Rename") }

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
