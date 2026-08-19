package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/trickylab/envii/internal/model"
	"github.com/trickylab/envii/internal/runner"
)

// exitFunc is os.Exit; overridden in tests.
var exitFunc = os.Exit

// runCmd: envii run <project> <env> -- <command...>
func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <project> <env> -- <command> [args...]",
		Short: "Run a command with the env's variables injected",
		Example: `  envii run my-api dev -- npm start
  envii run my-api prod -- go run ./cmd/server`,
		// Accept 2+ args; the command after -- is captured via ArgsLenAtDash.
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName, envName := args[0], args[1]

			// Support both styles:
			//   envii run proj env -- cmd arg   (preferred)
			//   envii run proj env cmd arg       (legacy)
			var argv []string
			if dash := cmd.ArgsLenAtDash(); dash >= 0 {
				argv = args[dash:]
			} else if len(args) > 2 {
				argv = args[2:]
			} else {
				return fmt.Errorf("no command provided — use: envii run %s %s -- <command>", projectName, envName)
			}

			v, _, _, err := loadVault()
			if err != nil {
				return err
			}
			env, err := resolveEnv(v, projectName, envName)
			if err != nil {
				return err
			}

			code, err := runner.Run(env, argv)
			if err != nil {
				return err
			}
			exitFunc(code)
			return nil
		},
	}
	return cmd
}

// exportCmd: envii export <project> <env> [-o file] [--resolve]
func exportCmd() *cobra.Command {
	var out string
	var resolve bool
	cmd := &cobra.Command{
		Use:   "export <project> <env>",
		Short: "Export an env as a .env file (stdout by default)",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			v, _, _, err := loadVault()
			if err != nil {
				return err
			}
			env, err := lookupEnv(v, args[0], args[1], resolve)
			if err != nil {
				return err
			}

			content := runner.Dotenv(env)
			if out == "" {
				fmt.Fprint(defaultIO.Stdout(), content)
				return nil
			}
			if err := os.WriteFile(out, []byte(content), 0o600); err != nil {
				return fmt.Errorf("write %s: %w", out, err)
			}
			fmt.Fprintf(defaultIO.Stderr(), "wrote %s\n", out)
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "out", "o", "", "output file (default: stdout)")
	cmd.Flags().BoolVar(&resolve, "resolve", false, "resolve env inheritance (base chain) before use")
	return cmd
}

// lookupEnv resolves a project/env pair, optionally folding the inheritance chain.
func lookupEnv(v *model.Vault, projectName, envName string, resolve bool) (*model.Env, error) {
	p := v.FindProject(projectName)
	if p == nil {
		return nil, fmt.Errorf("project %q not found", projectName)
	}
	if resolve {
		return p.ResolveEnv(envName)
	}
	e := p.FindEnv(envName)
	if e == nil {
		return nil, fmt.Errorf("env %q not found in project %q", envName, projectName)
	}
	return e, nil
}

// importCmd: envii import -f .env.production [--overwrite]
func importCmd() *cobra.Command {
	var file string
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "import -f <file>",
		Short: "Import variables from a .env file into the vault",
		Example: `  envii import -f .env
  envii import -f .env.production --overwrite`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if file == "" {
				return fmt.Errorf("missing file: use -f <file>")
			}

			f, err := os.Open(file)
			if err != nil {
				return fmt.Errorf("open %s: %w", file, err)
			}
			defer f.Close()

			vars, err := runner.ParseDotenv(f)
			if err != nil {
				return fmt.Errorf("parse %s: %w", file, err)
			}
			if len(vars) == 0 {
				return fmt.Errorf("%s contains no variables", file)
			}

			v, s, pass, err := loadVault()
			if err != nil {
				return err
			}

			in := bufio.NewReader(defaultIO.Stdin())
			project, err := promptProject(in, v)
			if err != nil {
				return err
			}
			env, err := promptEnv(in, project)
			if err != nil {
				return err
			}

			summary := model.ImportVars(env, vars, overwrite)
			if summary.Added == 0 && summary.Overwritten == 0 {
				fmt.Fprintf(defaultIO.Stderr(), "imported into %s/%s: 0 added, 0 overwritten, %d skipped; vault unchanged\n", project.Name, env.Name, summary.Skipped)
				return nil
			}
			if err := s.Save(v, pass); err != nil {
				return err
			}
			fmt.Fprintf(defaultIO.Stderr(), "imported into %s/%s: %d added, %d overwritten, %d skipped\n", project.Name, env.Name, summary.Added, summary.Overwritten, summary.Skipped)
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", ".env file to import")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "replace existing variables with matching keys")
	return cmd
}

func promptProject(in *bufio.Reader, vault *model.Vault) (*model.Project, error) {
	fmt.Fprintln(defaultIO.Stderr(), "Select project or enter a new name:")
	for i, p := range vault.Projects {
		fmt.Fprintf(defaultIO.Stderr(), "  %d) %s\n", i+1, p.Name)
	}
	choice, err := promptLine(in, "Project: ")
	if err != nil {
		return nil, err
	}
	if p, ok := projectChoice(vault, choice); ok {
		return p, nil
	}
	name := strings.TrimSpace(choice)
	if name == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}
	if p := vault.FindProject(name); p != nil {
		return p, nil
	}
	p := &model.Project{Name: name}
	vault.Projects = append(vault.Projects, p)
	return p, nil
}

func promptEnv(in *bufio.Reader, project *model.Project) (*model.Env, error) {
	fmt.Fprintf(defaultIO.Stderr(), "Select env for %s or enter a new name:\n", project.Name)
	for i, e := range project.Envs {
		fmt.Fprintf(defaultIO.Stderr(), "  %d) %s\n", i+1, e.Name)
	}
	choice, err := promptLine(in, "Env: ")
	if err != nil {
		return nil, err
	}
	if e, ok := envChoice(project, choice); ok {
		return e, nil
	}
	name := strings.TrimSpace(choice)
	if name == "" {
		return nil, fmt.Errorf("env name cannot be empty")
	}
	if e := project.FindEnv(name); e != nil {
		return e, nil
	}
	e := &model.Env{Name: name}
	project.Envs = append(project.Envs, e)
	return e, nil
}

func promptLine(in *bufio.Reader, label string) (string, error) {
	fmt.Fprint(defaultIO.Stderr(), label)
	line, err := in.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

func projectChoice(vault *model.Vault, choice string) (*model.Project, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(choice))
	if err != nil || n < 1 || n > len(vault.Projects) {
		return nil, false
	}
	return vault.Projects[n-1], true
}

func envChoice(project *model.Project, choice string) (*model.Env, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(choice))
	if err != nil || n < 1 || n > len(project.Envs) {
		return nil, false
	}
	return project.Envs[n-1], true
}
