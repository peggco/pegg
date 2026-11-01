package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	json "github.com/goccy/go-json"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/skillhub"
)

func (c *CLI) newSkillsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Browse and install skills from the Pegg hub",
	}
	cmd.AddCommand(
		c.newSkillsAddCommand(),
		c.newSkillsListCommand(),
		c.newSkillsSearchCommand(),
		c.newSkillsRemoveCommand(),
	)
	return cmd
}

func (c *CLI) newSkillsAddCommand() *cobra.Command {
	var project, global bool
	cmd := &cobra.Command{
		Use:   "add <owner>/<repo>/<skill>",
		Short: "Install a skill from the hub",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runSkillsAdd(cmd.Context(), cmd, cmd.OutOrStdout(), args[0], project, global)
		},
	}
	cmd.Flags().BoolVar(&project, "project", false, "install into the current project (.pegg/skills)")
	cmd.Flags().BoolVar(&global, "global", false, "install into the global skills directory (~/.pegg/skills)")
	cmd.MarkFlagsMutuallyExclusive("project", "global")
	return cmd
}

func (c *CLI) runSkillsAdd(ctx context.Context, cmd *cobra.Command, out io.Writer, ref string, project, global bool) error {
	spec, err := skillhub.ParseSpec(ref)
	if err != nil {
		return fmt.Errorf("cli: skills: %w", err)
	}
	isProject, err := c.resolveSkillScope(cmd, out, project, global)
	if err != nil {
		return err
	}
	dir, err := skillDir(isProject)
	if err != nil {
		return fmt.Errorf("cli: skills: %w", err)
	}
	client := skillhub.NewClient()
	installed, err := skillhub.Install(ctx, client, spec, dir)
	if err != nil {
		return fmt.Errorf("cli: skills: %w", err)
	}
	fmt.Fprintf(out, "Installed %s (%s) -> %s\n", installed.Name, scopeLabel(isProject), installed.Path)
	return nil
}

func (c *CLI) newSkillsListCommand() *cobra.Command {
	var project, global, asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runSkillsList(cmd.OutOrStdout(), project, global, asJSON)
		},
	}
	cmd.Flags().BoolVar(&project, "project", false, "only list project skills")
	cmd.Flags().BoolVar(&global, "global", false, "only list global skills")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.MarkFlagsMutuallyExclusive("project", "global")
	return cmd
}

func (c *CLI) runSkillsList(out io.Writer, project, global, asJSON bool) error {
	items, err := installedSkills(project, global)
	if err != nil {
		return fmt.Errorf("cli: skills: %w", err)
	}
	if asJSON {
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	if len(items) == 0 {
		fmt.Fprintln(out, "no skills installed")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSCOPE\tSOURCE\tPATH")
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.Name, item.Scope, item.Source, item.Path)
	}
	return w.Flush()
}

func (c *CLI) newSkillsSearchCommand() *cobra.Command {
	var limit int
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the Pegg skill hub",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runSkillsSearch(cmd.Context(), cmd.OutOrStdout(), args[0], limit, asJSON)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of results")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (c *CLI) runSkillsSearch(ctx context.Context, out io.Writer, query string, limit int, asJSON bool) error {
	items, err := skillhub.NewClient().Search(ctx, query, limit)
	if err != nil {
		return fmt.Errorf("cli: skills: %w", err)
	}
	if asJSON {
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	if len(items) == 0 {
		fmt.Fprintln(out, "no skills found")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tINSTALL")
	for _, item := range items {
		fmt.Fprintf(w, "%s\tpegg skills add %s/%s\n", item.Name, item.Source, item.SkillID)
	}
	return w.Flush()
}

func (c *CLI) newSkillsRemoveCommand() *cobra.Command {
	var project, global bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an installed skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runSkillsRemove(cmd.OutOrStdout(), args[0], project, global)
		},
	}
	cmd.Flags().BoolVar(&project, "project", false, "remove from the current project")
	cmd.Flags().BoolVar(&global, "global", false, "remove from the global skills directory")
	cmd.MarkFlagsMutuallyExclusive("project", "global")
	return cmd
}

func (c *CLI) runSkillsRemove(out io.Writer, name string, project, global bool) error {
	dirs, err := skillScopeDirs(project, global)
	if err != nil {
		return fmt.Errorf("cli: skills: %w", err)
	}
	for _, item := range dirs {
		err := skillhub.Remove(item.dir, name)
		if err == nil {
			fmt.Fprintf(out, "Removed %s (%s)\n", name, item.scope)
			return nil
		}
		if !errors.Is(err, skillhub.ErrSkillNotFound) {
			return fmt.Errorf("cli: skills: %w", err)
		}
	}
	return fmt.Errorf("cli: skills: %w: %s", skillhub.ErrSkillNotFound, name)
}

func (c *CLI) resolveSkillScope(cmd *cobra.Command, out io.Writer, project, global bool) (bool, error) {
	switch {
	case project:
		return true, nil
	case global:
		return false, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, nil
	}
	globalDir, _ := config.GetConfigPath("skills")
	projectDir, _ := skillDir(true)
	fmt.Fprintln(out, "Install scope:")
	fmt.Fprintf(out, "  1) global  (%s)\n", globalDir)
	fmt.Fprintf(out, "  2) project (%s)\n", projectDir)
	fmt.Fprint(out, "Choice [1]: ")
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("cli: skills: read scope: %w", err)
	}
	return parseSkillScopeChoice(line), nil
}

func parseSkillScopeChoice(choice string) bool {
	return strings.TrimSpace(choice) == "2"
}

func skillDir(project bool) (string, error) {
	if project {
		root, err := config.ProjectLocalRoot()
		if err != nil {
			return "", err
		}
		return filepath.Join(root, "skills"), nil
	}
	return config.GetConfigPath("skills")
}

func scopeLabel(project bool) string {
	if project {
		return "project"
	}
	return "global"
}

type skillScopeDir struct {
	dir   string
	scope string
}

func skillScopeDirs(project, global bool) ([]skillScopeDir, error) {
	var out []skillScopeDir
	seen := map[string]bool{}
	add := func(dir, scope string) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		out = append(out, skillScopeDir{dir: dir, scope: scope})
	}
	if !project {
		if home, err := os.UserHomeDir(); err == nil {
			add(filepath.Join(home, ".agents", "skills"), "agents")
		}
		if dir, err := config.GetConfigPath("skills"); err == nil {
			add(dir, "global")
		}
	}
	if !global {
		if root, err := config.ProjectLocalRoot(); err == nil {
			add(filepath.Join(root, "skills"), "project")
		}
		for _, dir := range config.ProjectReadPaths("skills") {
			add(dir, "project")
		}
	}
	return out, nil
}

func installedSkills(project, global bool) ([]skillhub.Installed, error) {
	dirs, err := skillScopeDirs(project, global)
	if err != nil {
		return nil, err
	}
	var out []skillhub.Installed
	for _, item := range dirs {
		out = append(out, skillhub.ListInstalled(item.dir, item.scope)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
