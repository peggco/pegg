package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

func (c *CLI) pluginCommands() []*cobra.Command {
	if c.pluginMgr == nil {
		return nil
	}
	var out []*cobra.Command
	for _, pc := range c.pluginMgr.Commands() {
		out = append(out, c.commandSpecToCobra(pc.Plugin, nil, pc.Spec))
	}
	return out
}

func (c *CLI) commandSpecToCobra(pluginName string, parentPath []string, spec pkgplugin.CommandSpec) *cobra.Command {
	name := commandName(spec.Use)
	path := append(append([]string(nil), parentPath...), name)
	cmd := &cobra.Command{
		Use:   spec.Use,
		Short: spec.Short,
		Long:  spec.Long,
	}
	for _, f := range spec.Flags {
		if f.Bool {
			def, _ := strconv.ParseBool(f.Default)
			cmd.Flags().BoolP(f.Name, f.Shorthand, def, f.Usage)
			continue
		}
		cmd.Flags().StringP(f.Name, f.Shorthand, f.Default, f.Usage)
	}
	if len(spec.Children) > 0 {
		for _, child := range spec.Children {
			cmd.AddCommand(c.commandSpecToCobra(pluginName, path, child))
		}
		return cmd
	}
	cmd.Args = func(cmd *cobra.Command, args []string) error {
		if len(args) < spec.MinArgs {
			return fmt.Errorf("requires at least %d arg(s)", spec.MinArgs)
		}
		if spec.MaxArgs > 0 && len(args) > spec.MaxArgs {
			return fmt.Errorf("accepts at most %d arg(s)", spec.MaxArgs)
		}
		return nil
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		flags := make(map[string]string, len(spec.Flags))
		for _, f := range spec.Flags {
			if f.Bool {
				v, _ := cmd.Flags().GetBool(f.Name)
				flags[f.Name] = strconv.FormatBool(v)
				continue
			}
			v, _ := cmd.Flags().GetString(f.Name)
			flags[f.Name] = v
		}
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		out, err := c.pluginMgr.RunCommand(ctx, pluginName, path, args, flags)
		if err != nil {
			return err
		}
		if out != "" {
			fmt.Fprint(cmd.OutOrStdout(), out)
		}
		return nil
	}
	return cmd
}

func commandName(use string) string {
	fields := strings.Fields(use)
	if len(fields) == 0 {
		return use
	}
	return fields[0]
}
