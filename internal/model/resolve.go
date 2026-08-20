package model

import "fmt"

// maxResolveDepth guards against accidental deep chains.
const maxResolveDepth = 8

// MergeEnvs overlays override's vars on top of base's vars.
// Keys present in override replace base entries entirely (value and Secret flag).
// Inputs are not mutated; a new Env is returned with override's Name.
func MergeEnvs(base, override *Env) *Env {
	merged := &Env{Name: override.Name, Base: override.Base, Vars: []*Var{}}
	index := make(map[string]int)
	for _, v := range base.Vars {
		cp := *v
		merged.Vars = append(merged.Vars, &cp)
		index[v.Key] = len(merged.Vars) - 1
	}
	for _, v := range override.Vars {
		cp := *v
		if i, ok := index[v.Key]; ok {
			merged.Vars[i] = &cp
		} else {
			merged.Vars = append(merged.Vars, &cp)
			index[v.Key] = len(merged.Vars) - 1
		}
	}
	return merged
}

// ResolveEnv returns the env with the given name, fully resolved by folding
// its Base chain (deepest base first, own vars winning last).
// Returns an error if the env is missing, the chain is too deep, or there is a cycle.
func (p *Project) ResolveEnv(name string) (*Env, error) {
	e := p.FindEnv(name)
	if e == nil {
		return nil, fmt.Errorf("env %q not found in project %q", name, p.Name)
	}
	chain := []*Env{}
	seen := map[string]bool{}
	for cur := e; cur != nil; {
		if len(chain) >= maxResolveDepth {
			return nil, fmt.Errorf("inheritance chain too deep at env %q", cur.Name)
		}
		if seen[cur.Name] {
			return nil, fmt.Errorf("inheritance cycle detected at env %q in project %q", cur.Name, p.Name)
		}
		seen[cur.Name] = true
		chain = append(chain, cur)
		if cur.Base == "" {
			break
		}
		cur = p.FindEnv(cur.Base)
		if cur == nil {
			return nil, fmt.Errorf("base env %q not found in project %q", chain[len(chain)-1].Base, p.Name)
		}
	}
	result := chain[len(chain)-1]
	for i := len(chain) - 2; i >= 0; i-- {
		result = MergeEnvs(result, chain[i])
	}
	return result, nil
}
