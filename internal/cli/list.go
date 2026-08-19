package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/trickylab/envii/internal/model"
)

// projectNames returns sorted project names.
func projectNames(v *model.Vault) []string {
	names := make([]string, 0, len(v.Projects))
	for _, p := range v.Projects {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names
}

// envNames returns sorted env names of a project.
func envNames(p *model.Project) []string {
	names := make([]string, 0, len(p.Envs))
	for _, e := range p.Envs {
		names = append(names, e.Name)
	}
	sort.Strings(names)
	return names
}

// keyNames returns sorted var keys of an env.
func keyNames(e *model.Env) []string {
	names := make([]string, 0, len(e.Vars))
	for _, v := range e.Vars {
		names = append(names, v.Key)
	}
	sort.Strings(names)
	return names
}

// lsCmd: envii ls [project] [env]
func lsCmd() *cobra.Command {
	var long bool
	cmd := &cobra.Command{
		Use:   "ls [project] [env]",
		Short: "List projects, environments, or keys",
		Args:  cobra.MaximumNArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			v, _, _, err := loadVault()
			if err != nil {
				return err
			}
			out := defaultIO.Stdout()
			switch len(args) {
			case 0:
				for _, name := range projectNames(v) {
					fmt.Fprintln(out, name)
				}
				return nil
			case 1:
				p := v.FindProject(args[0])
				if p == nil {
					return fmt.Errorf("project %q not found", args[0])
				}
				for _, name := range envNames(p) {
					fmt.Fprintln(out, name)
				}
				return nil
			default:
				env, err := resolveEnv(v, args[0], args[1])
				if err != nil {
					return err
				}
				vars := make([]*model.Var, len(env.Vars))
				copy(vars, env.Vars)
				sort.Slice(vars, func(i, j int) bool { return vars[i].Key < vars[j].Key })
				for _, v2 := range vars {
					if long && v2.Secret {
						fmt.Fprintf(out, "%s *\n", v2.Key)
						continue
					}
					fmt.Fprintln(out, v2.Key)
				}
				return nil
			}
		},
	}
	cmd.Flags().BoolVarP(&long, "long", "l", false, "show secret markers and base envs")
	return cmd
}
