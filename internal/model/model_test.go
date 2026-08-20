package model

import "testing"

func TestNewVault(t *testing.T) {
	v := NewVault()
	if v.Version != 1 {
		t.Fatalf("version got %d, want 1", v.Version)
	}
	if v.UpdatedAt.IsZero() {
		t.Fatal("updated_at should be set")
	}
	if v.Projects == nil {
		t.Fatal("projects should be an empty slice, not nil")
	}
	if len(v.Projects) != 0 {
		t.Fatalf("projects got %d, want 0", len(v.Projects))
	}
}

func TestFindProject(t *testing.T) {
	api := &Project{Name: "api"}
	web := &Project{Name: "web"}
	v := &Vault{Projects: []*Project{api, web}}

	if got := v.FindProject("web"); got != web {
		t.Fatalf("got %+v, want web project", got)
	}
	if got := v.FindProject("missing"); got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
	if got := (&Vault{}).FindProject("api"); got != nil {
		t.Fatalf("empty vault got %+v, want nil", got)
	}
}

func TestFindEnv(t *testing.T) {
	dev := &Env{Name: "dev"}
	prod := &Env{Name: "prod"}
	p := &Project{Envs: []*Env{dev, prod}}

	if got := p.FindEnv("prod"); got != prod {
		t.Fatalf("got %+v, want prod env", got)
	}
	if got := p.FindEnv("missing"); got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
	if got := (&Project{}).FindEnv("dev"); got != nil {
		t.Fatalf("empty project got %+v, want nil", got)
	}
}

func TestMap(t *testing.T) {
	env := &Env{Vars: []*Var{{Key: "A", Value: "one"}, {Key: "B", Value: "two"}, {Key: "A", Value: "last"}}}
	got := env.Map()

	if len(got) != 2 {
		t.Fatalf("got %d keys, want 2", len(got))
	}
	if got["A"] != "last" {
		t.Fatalf("duplicate key got %q, want last", got["A"])
	}
	if got["B"] != "two" {
		t.Fatalf("B got %q, want two", got["B"])
	}
}

func TestFindVar(t *testing.T) {
	env := &Env{Vars: []*Var{{Key: "PORT", Value: "8080"}, {Key: "TOKEN", Value: "x", Secret: true}}}

	if got := env.FindVar("PORT"); got == nil || got.Value != "8080" {
		t.Fatalf("FindVar(PORT) = %+v", got)
	}
	if got := env.FindVar("TOKEN"); got == nil || !got.Secret {
		t.Fatalf("FindVar(TOKEN) = %+v", got)
	}
	if got := env.FindVar("MISSING"); got != nil {
		t.Fatalf("FindVar(MISSING) = %+v, want nil", got)
	}
	empty := &Env{}
	if got := empty.FindVar("X"); got != nil {
		t.Fatalf("FindVar on empty env = %+v, want nil", got)
	}
}

func TestIsSecret(t *testing.T) {
	secretKeys := []string{
		"SECRET",
		"API_SECRET",
		"TOKEN",
		"ACCESS_TOKEN",
		"PASSWORD",
		"DB_PASSWORD",
		"KEY",
		"PRIVATE_KEY",
		"PRIVATE_VALUE",
		"CREDENTIAL",
		"AWS_CREDENTIALS",
		"API",
		"api_key",
	}
	for _, key := range secretKeys {
		t.Run("secret "+key, func(t *testing.T) {
			if !IsSecret(key) {
				t.Fatalf("expected %q to be secret", key)
			}
		})
	}

	nonSecretKeys := []string{"APP_NAME", "PORT", "HOST", "DATABASE_URL", "DEBUG", "LOG_LEVEL", ""}
	for _, key := range nonSecretKeys {
		t.Run("non-secret "+key, func(t *testing.T) {
			if IsSecret(key) {
				t.Fatalf("expected %q to be non-secret", key)
			}
		})
	}
}
