// Package model defines the core data structures for envii.
package model

import (
	"strings"
	"time"
)

// Vault is the top-level container persisted to disk (encrypted).
type Vault struct {
	Version   int        `json:"version"`
	UpdatedAt time.Time  `json:"updated_at"`
	Projects  []*Project `json:"projects"`
}

// Project groups related environments (e.g. "my-api").
type Project struct {
	Name string `json:"name"`
	Envs []*Env `json:"envs"`
}

// Env is a named set of variables (e.g. "dev", "staging", "prod").
// Base optionally names another env in the same project to inherit from.
type Env struct {
	Name string `json:"name"`
	Base string `json:"base,omitempty"`
	Vars []*Var `json:"vars"`
}

// Var is a single key/value pair. Secret marks values that should be
// masked in the UI by default.
type Var struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

// NewVault returns an empty vault with the current schema version.
// Optional VaultOption values (e.g. WithClock) configure construction.
func NewVault(opts ...VaultOption) *Vault {
	o := vaultOptions{clock: ClockFunc(time.Now)}
	for _, opt := range opts {
		opt(&o)
	}
	if o.clock == nil {
		o.clock = ClockFunc(time.Now)
	}
	return &Vault{Version: 1, UpdatedAt: o.clock.Now(), Projects: []*Project{}}
}

// FindProject returns the project with the given name, or nil.
func (v *Vault) FindProject(name string) *Project {
	for _, p := range v.Projects {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// FindEnv returns the env with the given name, or nil.
func (p *Project) FindEnv(name string) *Env {
	for _, e := range p.Envs {
		if e.Name == name {
			return e
		}
	}
	return nil
}

// Map converts an env's vars to a plain key/value map.
func (e *Env) Map() map[string]string {
	m := make(map[string]string, len(e.Vars))
	for _, v := range e.Vars {
		m[v.Key] = v.Value
	}
	return m
}

// IsSecret guesses whether a key likely holds a secret value.
func IsSecret(key string) bool {
	k := strings.ToUpper(key)
	for _, hint := range []string{"SECRET", "TOKEN", "PASSWORD", "KEY", "PRIVATE", "CREDENTIAL", "API"} {
		if strings.Contains(k, hint) {
			return true
		}
	}
	return false
}
