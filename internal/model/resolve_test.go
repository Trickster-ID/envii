package model

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestMergeEnvs(t *testing.T) {
	base := &Env{Name: "shared", Vars: []*Var{
		{Key: "DB_HOST", Value: "localhost"},
		{Key: "SECRET_A", Value: "base-secret", Secret: true},
	}}
	override := &Env{Name: "prod", Vars: []*Var{
		{Key: "DB_HOST", Value: "prod-db.internal"},
		{Key: "EXTRA", Value: "1"},
	}}

	got := MergeEnvs(base, override)

	if got.Name != "prod" {
		t.Fatalf("Name = %q", got.Name)
	}
	m := got.Map()
	if m["DB_HOST"] != "prod-db.internal" {
		t.Fatalf("override must win: %v", m)
	}
	if m["SECRET_A"] != "base-secret" {
		t.Fatalf("base-only vars must survive: %v", m)
	}
	if m["EXTRA"] != "1" {
		t.Fatalf("override-only vars must appear: %v", m)
	}
	if len(got.Vars) != 3 {
		t.Fatalf("len = %d, want 3", len(got.Vars))
	}
	if len(base.Vars) != 2 || len(override.Vars) != 2 {
		t.Fatal("inputs were mutated")
	}
	if got.Vars[0].Secret {
		t.Fatal("override entry at index 0 must be the copied base entry; DB_HOST override should not be secret")
	}
	if base.Vars[0].Value != "localhost" || override.Vars[0].Value != "prod-db.internal" {
		t.Fatal("input var values mutated")
	}
}

func TestResolveEnv(t *testing.T) {
	newEnv := func(name, base string, vars ...*Var) *Env {
		return &Env{Name: name, Base: base, Vars: vars}
	}
	proj := &Project{Name: "api", Envs: []*Env{
		newEnv("shared", "",
			&Var{Key: "DB_HOST", Value: "localhost"},
			&Var{Key: "PORT", Value: "3000"}),
		newEnv("dev", "shared",
			&Var{Key: "DEBUG", Value: "true"}),
		newEnv("prod", "dev",
			&Var{Key: "DB_HOST", Value: "prod-db"},
			&Var{Key: "DEBUG", Value: "false"}),
	}}

	tests := []struct {
		name    string
		envName string
		want    map[string]string
	}{
		{"no base", "shared", map[string]string{"DB_HOST": "localhost", "PORT": "3000"}},
		{"one level", "dev", map[string]string{"DB_HOST": "localhost", "PORT": "3000", "DEBUG": "true"}},
		{"two levels", "prod", map[string]string{"DB_HOST": "prod-db", "PORT": "3000", "DEBUG": "false"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := proj.ResolveEnv(tt.envName)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Map(), tt.want) {
				t.Fatalf("got %v, want %v", got.Map(), tt.want)
			}
		})
	}
}

func TestResolveEnvErrors(t *testing.T) {
	newEnv := func(name, base string) *Env { return &Env{Name: name, Base: base} }

	deepChain := make([]*Env, 0, 10)
	for i := 1; i <= 10; i++ {
		base := ""
		if i > 1 {
			base = fmt.Sprintf("env%02d", i-1)
		}
		deepChain = append(deepChain, newEnv(fmt.Sprintf("env%02d", i), base))
	}

	tests := []struct {
		name    string
		envs    []*Env
		target  string
		wantErr string
	}{
		{"missing env", []*Env{newEnv("a", "")}, "zzz", `env "zzz" not found`},
		{"missing base", []*Env{newEnv("a", "ghost")}, "a", `base env "ghost" not found`},
		{"self cycle", []*Env{newEnv("a", "a")}, "a", "cycle detected"},
		{"two-node cycle", []*Env{newEnv("a", "b"), newEnv("b", "a")}, "a", "cycle detected"},
		{"deep chain", deepChain, "env10", "inheritance chain too deep"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Project{Name: "p", Envs: tt.envs}
			_, err := p.ResolveEnv(tt.target)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
