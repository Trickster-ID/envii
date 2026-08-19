package cli

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/trickylab/envii/internal/model"
)

func TestProjectNames(t *testing.T) {
	tests := []struct {
		name  string
		vault *model.Vault
		want  []string
	}{
		{"empty", &model.Vault{}, []string{}},
		{"single", &model.Vault{Projects: []*model.Project{{Name: "api"}}}, []string{"api"}},
		{"multi sorted", &model.Vault{Projects: []*model.Project{{Name: "web"}, {Name: "api"}, {Name: "db"}}}, []string{"api", "db", "web"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := projectNames(tt.vault); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestEnvNames(t *testing.T) {
	tests := []struct {
		name    string
		project *model.Project
		want    []string
	}{
		{"empty", &model.Project{Name: "api"}, []string{}},
		{"single", &model.Project{Name: "api", Envs: []*model.Env{{Name: "dev"}}}, []string{"dev"}},
		{"multi sorted", &model.Project{Name: "api", Envs: []*model.Env{{Name: "prod"}, {Name: "dev"}}}, []string{"dev", "prod"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := envNames(tt.project); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestKeyNames(t *testing.T) {
	tests := []struct {
		name string
		env  *model.Env
		want []string
	}{
		{"empty", &model.Env{Name: "dev"}, []string{}},
		{"single", &model.Env{Name: "dev", Vars: []*model.Var{{Key: "PORT"}}}, []string{"PORT"}},
		{"multi sorted", &model.Env{Name: "dev", Vars: []*model.Var{{Key: "TOKEN"}, {Key: "PORT"}}}, []string{"PORT", "TOKEN"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := keyNames(tt.env); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestLsCmd(t *testing.T) {
	pass := "secret"
	path := filepath.Join(t.TempDir(), "vault.age")
	seedVault(t, path, pass, sampleVault())
	withVaultPath(t, path)

	run := func(args ...string) (string, error) {
		t.Helper()
		out := &bytes.Buffer{}
		withIO(t, &fakeIO{
			env:    map[string]string{"ENVII_PASSPHRASE": pass},
			stdin:  &bytes.Buffer{},
			stdout: out,
			stderr: &bytes.Buffer{},
		})
		cmd := lsCmd()
		cmd.SetArgs(args)
		err := cmd.Execute()
		return out.String(), err
	}

	t.Run("projects", func(t *testing.T) {
		out, err := run()
		if err != nil {
			t.Fatal(err)
		}
		if out != "api\n" {
			t.Fatalf("out %q", out)
		}
	})

	t.Run("missing project", func(t *testing.T) {
		_, err := run("missing")
		if err == nil || !strings.Contains(err.Error(), `project "missing" not found`) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("envs", func(t *testing.T) {
		out, err := run("api")
		if err != nil {
			t.Fatal(err)
		}
		if out != "dev\n" {
			t.Fatalf("out %q", out)
		}
	})

	t.Run("keys", func(t *testing.T) {
		out, err := run("api", "dev")
		if err != nil {
			t.Fatal(err)
		}
		if out != "PORT\nTOKEN\n" {
			t.Fatalf("out %q", out)
		}
	})

	t.Run("keys long secret marker", func(t *testing.T) {
		out, err := run("--long", "api", "dev")
		if err != nil {
			t.Fatal(err)
		}
		if out != "PORT\nTOKEN *\n" {
			t.Fatalf("out %q", out)
		}
	})

	t.Run("env long", func(t *testing.T) {
		out, err := run("--long", "api")
		if err != nil {
			t.Fatal(err)
		}
		if out != "dev\n" {
			t.Fatalf("out %q", out)
		}
	})

	t.Run("missing env", func(t *testing.T) {
		_, err := run("api", "prod")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("too many args", func(t *testing.T) {
		_, err := run("a", "b", "c", "d")
		if err == nil {
			t.Fatal("expected arg error")
		}
	})
}

func TestCompleteVault(t *testing.T) {
	pass := "secret"
	path := filepath.Join(t.TempDir(), "vault.age")
	seedVault(t, path, pass, sampleVault())
	withVaultPath(t, path)
	withIO(t, &fakeIO{
		env:    map[string]string{"ENVII_PASSPHRASE": pass},
		stdin:  &bytes.Buffer{},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	})

	tests := []struct {
		name string
		pos  int
		args []string
		want []string
	}{
		{"projects", 0, nil, []string{"api"}},
		{"envs", 1, []string{"api"}, []string{"dev"}},
		{"keys", 2, []string{"api", "dev"}, []string{"PORT", "TOKEN"}},
		{"wrong pos", 0, []string{"api"}, nil},
		{"missing project", 1, []string{"web"}, nil},
		{"missing env", 2, []string{"api", "prod"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := completeVault(tt.pos)(nil, tt.args, "")
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestCompleteVaultLoadError(t *testing.T) {
	withVaultPath(t, filepath.Join(t.TempDir(), "missing.age"))
	withIO(t, &fakeIO{
		env:    map[string]string{"ENVII_PASSPHRASE": "x"},
		stdin:  &bytes.Buffer{},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	})
	got, _ := completeVault(0)(nil, nil, "")
	if got != nil {
		t.Fatalf("got %v want nil", got)
	}
}
