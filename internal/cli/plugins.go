package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/plugin"
	"github.com/peggco/pegg/internal/plugin/install"
)

type pluginListEntry struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Source      string `json:"source,omitempty"`
	Status      string `json:"status"`
	Path        string `json:"path"`
	Asset       string `json:"asset,omitempty"`
	InstalledAt string `json:"installed_at,omitempty"`
	Manual      bool   `json:"manual,omitempty"`
}

func (c *CLI) runPluginsList(cmd *cobra.Command, asJSON bool) error {
	pluginDir, err := plugin.GetPluginDir()
	if err != nil {
		return err
	}
	items, err := install.List(pluginDir)
	if err != nil {
		return err
	}
	excluded := map[string]bool{}
	currentConfig := c.cfg
	if loaded, err := config.Load(); err == nil {
		currentConfig = loaded
	}
	if currentConfig != nil {
		for _, name := range currentConfig.Plugins.Exclude {
			excluded[name] = true
		}
	}
	entries := make([]pluginListEntry, 0, len(items))
	for _, item := range items {
		status := "enabled"
		if excluded[item.Name] || (item.Source != "" && excluded[item.Source]) {
			status = "excluded"
		} else if item.Source != "" {
			if source, err := install.ParseSource(item.Source); err == nil && excluded[source.Repo] {
				status = "excluded"
			}
		}
		entry := pluginListEntry{
			Name:    item.Name,
			Version: item.Version,
			Source:  item.Source,
			Status:  status,
			Path:    item.Path,
			Asset:   item.Asset,
			Manual:  item.Manual,
		}
		if !item.InstalledAt.IsZero() {
			entry.InstalledAt = item.InstalledAt.UTC().Format(time.RFC3339)
		}
		entries = append(entries, entry)
	}

	if asJSON {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(entries)
	}
	if len(entries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No plugins installed.")
		fmt.Fprintf(cmd.OutOrStdout(), "Plugin directory: %s\n", pluginDir)
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tSOURCE\tSTATUS")
	fmt.Fprintln(w, "----\t-------\t------\t------")
	for _, entry := range entries {
		version := entry.Version
		if version == "" {
			version = "-"
		}
		source := entry.Source
		if source == "" {
			source = "manual"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", entry.Name, version, source, entry.Status)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %d plugins\n", len(entries))
	return nil
}

func (c *CLI) newPluginsInstallCommand() *cobra.Command {
	var opts install.Options
	cmd := &cobra.Command{
		Use:   "install <owner/repo>",
		Short: "Install a plugin from GitHub releases",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source, err := install.ParseSource(args[0])
			if err != nil {
				return err
			}
			opts.Source = source
			opts.Out = cmd.OutOrStdout()
			dir, err := plugin.GetPluginDir()
			if err != nil {
				return err
			}
			opts.Dir = dir
			result, err := install.Install(cliContext(cmd), install.NewClient(), opts)
			if err != nil {
				return err
			}
			printInstallResult(cmd, result, false)
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Version, "version", "", "release tag to install (default: latest)")
	cmd.Flags().StringVar(&opts.AssetPattern, "asset", "", "release asset name or glob")
	cmd.Flags().StringVar(&opts.BinaryPath, "binary", "", "binary path inside an archive")
	cmd.Flags().StringVar(&opts.Name, "name", "", "installed binary name")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "replace an existing plugin")
	cmd.Flags().BoolVar(&opts.SkipChecksum, "skip-checksum", false, "skip checksum verification")
	cmd.Flags().BoolVar(&opts.RequireChecksum, "require-checksum", false, "fail when no checksum is available")
	return cmd
}

func (c *CLI) newPluginsUpdateCommand() *cobra.Command {
	var opts install.Options
	var all, check bool
	cmd := &cobra.Command{
		Use:   "update [owner/repo]",
		Short: "Update plugins installed from GitHub releases",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return fmt.Errorf("use either a plugin name or --all, not both")
			}
			if !all && len(args) == 0 {
				return fmt.Errorf("specify a plugin or use --all")
			}
			dir, err := plugin.GetPluginDir()
			if err != nil {
				return err
			}
			client := install.NewClient()
			opts.Out = cmd.OutOrStdout()
			opts.Dir = dir
			opts.Check = check

			if all {
				metas, err := install.ListMetadata(dir)
				if err != nil {
					return err
				}
				if len(metas) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No GitHub-installed plugins to update.")
					return nil
				}
				var failures []string
				for _, meta := range metas {
					source, err := install.ParseSource(meta.Source)
					if err != nil {
						failures = append(failures, fmt.Sprintf("%s: %v", meta.Name, err))
						continue
					}
					opts.Source = source
					result, err := install.Update(cliContext(cmd), client, opts)
					if err != nil {
						failures = append(failures, fmt.Sprintf("%s: %v", meta.Name, err))
						continue
					}
					printUpdateResult(cmd, result)
				}
				if len(failures) > 0 {
					return fmt.Errorf("some plugins failed to update:\n%s", strings.Join(failures, "\n"))
				}
				return nil
			}

			source, err := resolveUpdateSource(dir, args[0])
			if err != nil {
				return err
			}
			opts.Source = source
			result, err := install.Update(cliContext(cmd), client, opts)
			if err != nil {
				return err
			}
			printUpdateResult(cmd, result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "update all GitHub-installed plugins")
	cmd.Flags().BoolVar(&check, "check", false, "only report whether an update is available")
	cmd.Flags().StringVar(&opts.AssetPattern, "asset", "", "release asset name or glob")
	cmd.Flags().StringVar(&opts.BinaryPath, "binary", "", "binary path inside an archive")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "reinstall even when already up to date")
	cmd.Flags().BoolVar(&opts.SkipChecksum, "skip-checksum", false, "skip checksum verification")
	cmd.Flags().BoolVar(&opts.RequireChecksum, "require-checksum", false, "fail when no checksum is available")
	return cmd
}

func (c *CLI) newPluginsRemoveCommand() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <name|owner/repo>",
		Short: "Remove an installed plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := plugin.GetPluginDir()
			if err != nil {
				return err
			}
			item, err := install.Resolve(dir, args[0])
			if err != nil {
				return err
			}
			aliases := pluginAliases(dir, args[0])
			if !yes {
				ok, err := confirm(cmd, fmt.Sprintf("Remove plugin %s?", item.Name))
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
					return nil
				}
			}
			if err := install.Remove(dir, args[0]); err != nil {
				return err
			}
			_, _ = config.IncludePlugin(aliases...)
			fmt.Fprintf(cmd.OutOrStdout(), "Removed plugin %s\n", item.Name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "remove without confirmation")
	return cmd
}

func (c *CLI) newPluginsExcludeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "exclude <name|owner/repo>...",
		Short: "Disable plugins by adding them to plugins.exclude",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := plugin.GetPluginDir()
			if err != nil {
				return err
			}
			var names []string
			for _, arg := range args {
				names = append(names, pluginAliases(dir, arg)...)
			}
			changed, err := config.ExcludePlugin(names...)
			if err != nil {
				return err
			}
			if changed {
				fmt.Fprintf(cmd.OutOrStdout(), "Excluded: %s\n", strings.Join(names, ", "))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Plugins are already excluded.")
			}
			return nil
		},
	}
}

func (c *CLI) newPluginsIncludeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "include <name|owner/repo>...",
		Short: "Enable plugins by removing them from plugins.exclude",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := plugin.GetPluginDir()
			if err != nil {
				return err
			}
			var names []string
			for _, arg := range args {
				names = append(names, pluginAliases(dir, arg)...)
			}
			changed, err := config.IncludePlugin(names...)
			if err != nil {
				return err
			}
			if changed {
				fmt.Fprintf(cmd.OutOrStdout(), "Included: %s\n", strings.Join(names, ", "))
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Plugins were not excluded.")
			}
			return nil
		},
	}
}

func resolveUpdateSource(dir, value string) (install.Source, error) {
	if strings.Contains(value, "/") {
		return install.ParseSource(value)
	}
	item, err := install.Resolve(dir, value)
	if err != nil {
		return install.Source{}, err
	}
	if item.Source == "" {
		return install.Source{}, fmt.Errorf("plugin %q was not installed by `pegg plugins install`", value)
	}
	return install.ParseSource(item.Source)
}

func pluginAliases(dir, value string) []string {
	seen := map[string]bool{}
	var aliases []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name != "" && !seen[name] {
			seen[name] = true
			aliases = append(aliases, name)
		}
	}
	if item, err := install.Resolve(dir, value); err == nil {
		add(item.Name)
		add(item.Source)
		if item.Source != "" {
			if source, err := install.ParseSource(item.Source); err == nil {
				add(source.Repo)
			}
		}
		return aliases
	}
	if strings.Contains(value, "/") {
		if source, err := install.ParseSource(value); err == nil {
			add(source.Slug())
			add(source.Repo)
			return aliases
		}
	}
	add(value)
	return aliases
}

func printInstallResult(cmd *cobra.Command, result *install.Result, update bool) {
	verb := "Installed"
	if update {
		verb = "Updated"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s from %s\n", verb, result.Name, result.Version, result.Source)
	fmt.Fprintf(cmd.OutOrStdout(), "Path: %s\n", result.Path)
	fmt.Fprintf(cmd.OutOrStdout(), "Asset: %s\n", result.Asset)
	fmt.Fprintf(cmd.OutOrStdout(), "SHA256: %s\n", result.SHA256)
}

func printUpdateResult(cmd *cobra.Command, result *install.Result) {
	if result.Updated {
		printInstallResult(cmd, result, true)
		return
	}
	if result.UpdateAvailable {
		from := result.PreviousVersion
		if from == "" {
			from = result.Tag
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Update available for %s: %s -> %s\n", result.Name, from, result.Version)
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s is up to date (%s)\n", result.Name, result.Version)
}

func cliContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

func confirm(cmd *cobra.Command, question string) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, fmt.Errorf("refusing to continue without --yes (stdin is not a terminal)")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N] ", question)
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
