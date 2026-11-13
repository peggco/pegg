package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type pluginNewOptions struct {
	dir     string
	module  string
	name    string
	pegg    string
	version string
	replace string
}

func (c *CLI) newPluginsNewCommand() *cobra.Command {
	opts := pluginNewOptions{}
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a new Pegg plugin repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = strings.TrimSpace(args[0])
			if opts.dir == "" {
				opts.dir = opts.name
			}
			if opts.module == "" {
				opts.module = opts.name
			}
			return scaffoldPlugin(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.dir, "dir", "", "target directory (defaults to the plugin name)")
	cmd.Flags().StringVar(&opts.module, "module", "", "Go module path (defaults to the plugin name)")
	cmd.Flags().StringVar(&opts.pegg, "pegg", "github.com/peggco/pegg", "Pegg module path")
	cmd.Flags().StringVar(&opts.version, "pegg-version", "v0.2.4", "Pegg version to require")
	cmd.Flags().StringVar(&opts.replace, "replace", "", "optional local Pegg path for a go.mod replace")
	return cmd
}

func scaffoldPlugin(cmd *cobra.Command, opts pluginNewOptions) error {
	if opts.name == "" {
		return fmt.Errorf("plugin name is required")
	}
	if _, err := os.Stat(opts.dir); err == nil {
		return fmt.Errorf("directory %q already exists", opts.dir)
	}
	if err := os.MkdirAll(opts.dir, 0o755); err != nil {
		return err
	}

	replace := ""
	if strings.TrimSpace(opts.replace) != "" {
		replace = "\nreplace " + opts.pegg + " => " + filepath.ToSlash(opts.replace) + "\n"
	}
	goMod := fmt.Sprintf("module %s\n\ngo 1.26.5\n\nrequire %s %s\n%s", opts.module, opts.pegg, opts.version, replace)

	mainGo := fmt.Sprintf(`package main

import (
	"os"

	peggplugin "%s/pkg/plugin"
)

func main() {
	if len(os.Args) > 1 && os.Getenv("PEGG_PLUGIN") == "" {
		os.Exit(runCLI(os.Args[1:]))
	}
	peggplugin.Serve(&Plugin{})
}
`, opts.pegg)

	pluginGo := fmt.Sprintf(`package main

import (
	"fmt"

	peggplugin "%s/pkg/plugin"
)

type Plugin struct {
	peggplugin.Base
}

func (p *Plugin) Name() string        { return %q }
func (p *Plugin) Version() string     { return "0.1.0" }
func (p *Plugin) Description() string { return "A Pegg plugin" }

func (p *Plugin) Capabilities() []string {
	return []string{string(peggplugin.CapabilityTools)}
}

func (p *Plugin) Tools() ([]peggplugin.ToolSpec, error) {
	schema := "{\"type\":\"object\",\"properties\":{\"text\":{\"type\":\"string\"}},\"required\":[\"text\"]}"
	return []peggplugin.ToolSpec{{
		Name:        %q,
		Description: "Example tool. Replace this with your plugin's tool.",
		Parameters:  schema,
		Bundle:      "example",
		Group:       "Example",
		ReadOnly:    true,
	}}, nil
}

func (p *Plugin) ExecuteTool(req peggplugin.ToolRequest) (*peggplugin.ToolResult, error) {
	if req.Name != %q {
		return nil, fmt.Errorf("unknown tool %%q", req.Name)
	}
	return &peggplugin.ToolResult{Output: "hello from %s"}, nil
}
`, opts.pegg, opts.name, opts.name+"_echo", opts.name+"_echo", opts.name)

	commandsGo := `package main

func runCLI(args []string) int {
	println("This plugin has no CLI commands yet.")
	return 0
}
`

	readme := fmt.Sprintf(`# %s

A Pegg plugin.

## Build

~~~bash
go mod tidy
go build -o bin/%s .
mkdir -p ~/.pegg/plugins
cp bin/%s ~/.pegg/plugins/%s
chmod +x ~/.pegg/plugins/%s
~~~

## Use

Load a skill whose frontmatter contains `+"`tools: example`"+`, then call the
`+"`%s_echo`"+` tool.
`, opts.name, opts.name, opts.name, opts.name, opts.name, opts.name)

	files := map[string]string{
		"go.mod":      goMod,
		"main.go":     mainGo,
		"plugin.go":   pluginGo,
		"commands.go": commandsGo,
		"README.md":   readme,
		".gitignore":  "bin/\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(opts.dir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created plugin %q in %s\n", opts.name, opts.dir)
	return nil
}
