package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/peggco/pegg/internal/agent/tools"
	"github.com/peggco/pegg/internal/catalog"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/mcp"
)

const (
	mcpScopeGlobal  = "global"
	mcpScopeProject = "project"
)

const (
	mcpTransportStdio          = "stdio"
	mcpTransportSSE            = "sse"
	mcpTransportStreamableHTTP = "streamable-http"
)

var popularHeaderKeys = []string{
	"Authorization",
	"X-API-Key",
	"Cookie",
	"User-Agent",
}

func (c *CLI) newMCPCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Manage MCP servers",
	}
	cmd.AddCommand(
		c.newMCPListCommand(),
		c.newMCPAddCommand(),
		c.newMCPRemoveCommand(),
		c.newMCPToolsCommand(),
		c.newMCPSearchCommand(),
		c.newMCPInfoCommand(),
		c.newMCPInstallCommand(),
	)
	return cmd
}

func (c *CLI) newMCPListCommand() *cobra.Command {
	var globalOnly, projectOnly bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List MCP servers (global and project)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runMCPList(cmd.OutOrStdout(), globalOnly, projectOnly)
		},
	}
	cmd.Flags().BoolVar(&globalOnly, "global", false, "only list global servers")
	cmd.Flags().BoolVar(&projectOnly, "project", false, "only list project servers")
	return cmd
}

func (c *CLI) runMCPList(out io.Writer, globalOnly, projectOnly bool) error {
	if globalOnly && projectOnly {
		return errors.New("cli: cannot combine --global and --project")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("cli: load config: %w", err)
	}
	project, err := mcp.LoadProjectServers(".")
	if err != nil {
		return fmt.Errorf("cli: load project servers: %w", err)
	}

	if globalOnly || projectOnly {
		servers := project
		label := "Project"
		if globalOnly {
			servers = cfg.MCPServers
			label = "Global"
		}
		fmt.Fprintf(out, "%s (%d):\n", label, len(servers))
		printMCPServers(out, servers)
		return nil
	}

	fmt.Fprintf(out, "Global (%d):\n", len(cfg.MCPServers))
	printMCPServers(out, cfg.MCPServers)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "Project (%d):\n", len(project))
	printMCPServers(out, project)
	return nil
}

func printMCPServers(out io.Writer, servers map[string]config.MCPServerConfig) {
	if len(servers) == 0 {
		fmt.Fprintln(out, "  (none)")
		return
	}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Fprintf(out, "  %-24s %-7s %s\n", "NAME", "TYPE", "DETAIL")
	for _, name := range names {
		s := servers[name]
		kind, err := mcp.ResolveTransportKind(s)
		if err != nil {
			fmt.Fprintf(out, "  %-24s %-7s %s\n", name, "invalid", "(invalid: no command or url)")
			continue
		}
		if kind == mcp.TransportStdio {
			detail := strings.TrimSpace(s.Command + " " + strings.Join(s.Args, " "))
			fmt.Fprintf(out, "  %-24s %-7s %s\n", name, string(kind), detail)
			continue
		}
		fmt.Fprintf(out, "  %-24s %-7s %s\n", name, string(kind), s.URL)
	}
}

type mcpAddFlags struct {
	globalOnly  bool
	projectOnly bool
	name        string
	transport   string
	command     string
	args        []string
	env         map[string]string
	url         string
	headers     map[string]string

	nameSet      bool
	transportSet bool
	commandSet   bool
	argsSet      bool
	envSet       bool
	urlSet       bool
	headersSet   bool
}

func (c *CLI) newMCPAddCommand() *cobra.Command {
	f := &mcpAddFlags{}
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an MCP server (interactive unless flags are given)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags := cmd.Flags()
			f.nameSet = flags.Changed("name")
			f.transportSet = flags.Changed("transport")
			f.commandSet = flags.Changed("command")
			f.argsSet = flags.Changed("arg")
			f.envSet = flags.Changed("env")
			f.urlSet = flags.Changed("url")
			f.headersSet = flags.Changed("header")
			return c.runMCPAdd(cmd.OutOrStdout(), f)
		},
	}
	cmd.Flags().BoolVar(&f.globalOnly, "global", false, "add to global config")
	cmd.Flags().BoolVar(&f.projectOnly, "project", false, "add to project .mcp.json")
	cmd.Flags().StringVar(&f.name, "name", "", "server name")
	cmd.Flags().StringVar(&f.transport, "transport", "", "transport type: stdio, sse (legacy), or streamable-http")
	cmd.Flags().StringVar(&f.command, "command", "", "local command to run")
	cmd.Flags().StringSliceVar(&f.args, "arg", nil, "local command argument (repeatable)")
	cmd.Flags().StringToStringVar(&f.env, "env", nil, "local environment variable KEY=VALUE (repeatable)")
	cmd.Flags().StringVar(&f.url, "url", "", "MCP endpoint (SSE or streamable HTTP)")
	cmd.Flags().StringToStringVar(&f.headers, "header", nil, "request header KEY=VALUE (repeatable)")
	return cmd
}

func (c *CLI) runMCPAdd(out io.Writer, f *mcpAddFlags) error {
	if f.globalOnly && f.projectOnly {
		return errors.New("cli: cannot combine --global and --project")
	}

	scope := ""
	switch {
	case f.globalOnly:
		scope = mcpScopeGlobal
	case f.projectOnly:
		scope = mcpScopeProject
	default:
		s, err := promptScope()
		if err != nil {
			return err
		}
		scope = s
	}

	transport := strings.ToLower(strings.TrimSpace(f.transport))
	if !f.transportSet || transport == "" {
		t, err := promptTransport()
		if err != nil {
			return err
		}
		transport = t
	}
	kind, err := mcp.ParseTransportKind(transport)
	if err != nil || kind == "" {
		return fmt.Errorf("cli: unknown transport %q (use stdio, sse (legacy), or streamable-http)", transport)
	}

	name := strings.TrimSpace(f.name)
	if !f.nameSet || name == "" {
		n, err := promptString("Server name")
		if err != nil {
			return err
		}
		name = n
	}

	var server config.MCPServerConfig
	switch kind {
	case mcp.TransportStdio:
		command := strings.TrimSpace(f.command)
		if !f.commandSet || command == "" {
			cmd, err := promptString("Command")
			if err != nil {
				return err
			}
			command = cmd
		}
		args := f.args
		if !f.argsSet || len(args) == 0 {
			a, err := promptArgs()
			if err != nil {
				return err
			}
			args = a
		}
		env := f.env
		if !f.envSet || len(env) == 0 {
			e, err := promptEnv()
			if err != nil {
				return err
			}
			env = e
		}
		server = config.MCPServerConfig{Type: string(kind), Command: command, Args: args, Env: env}
	case mcp.TransportSSE, mcp.TransportStreamableHTTP:
		url := strings.TrimSpace(f.url)
		if !f.urlSet || url == "" {
			u, err := promptString("URL")
			if err != nil {
				return err
			}
			url = u
		}
		headers := f.headers
		if !f.headersSet || len(headers) == 0 {
			h, err := promptHeaders()
			if err != nil {
				return err
			}
			headers = h
		}
		server = config.MCPServerConfig{Type: string(kind), URL: url, Headers: headers}
	}

	if scope == mcpScopeGlobal {
		if err := config.UpsertMCPServer(name, server); err != nil {
			return fmt.Errorf("cli: save mcp server: %w", err)
		}
	} else {
		if err := mcp.UpsertProjectServer(".", name, server); err != nil {
			return fmt.Errorf("cli: save project mcp server: %w", err)
		}
	}

	c.log.Finfo("mcp server %q saved (%s)", name, scope)
	fmt.Fprintf(out, "saved mcp server %q to %s\n", name, scope)
	return nil
}

func (c *CLI) newMCPRemoveCommand() *cobra.Command {
	var globalOnly, projectOnly bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runMCPRemove(cmd.OutOrStdout(), args[0], globalOnly, projectOnly)
		},
	}
	cmd.Flags().BoolVar(&globalOnly, "global", false, "remove from global config")
	cmd.Flags().BoolVar(&projectOnly, "project", false, "remove from project .mcp.json")
	return cmd
}

func (c *CLI) runMCPRemove(out io.Writer, name string, globalOnly, projectOnly bool) error {
	if globalOnly && projectOnly {
		return errors.New("cli: cannot combine --global and --project")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("cli: load config: %w", err)
	}
	project, err := mcp.LoadProjectServers(".")
	if err != nil {
		return fmt.Errorf("cli: load project servers: %w", err)
	}

	_, inGlobal := cfg.MCPServers[name]
	_, inProject := project[name]

	scope := ""
	switch {
	case globalOnly:
		scope = mcpScopeGlobal
	case projectOnly:
		scope = mcpScopeProject
	case inGlobal && !inProject:
		scope = mcpScopeGlobal
	case !inGlobal && inProject:
		scope = mcpScopeProject
	case inGlobal && inProject:
		s, err := promptScope()
		if err != nil {
			return err
		}
		scope = s
	default:
		return fmt.Errorf("cli: mcp server %q not found", name)
	}

	if scope == mcpScopeGlobal {
		if err := config.RemoveMCPServer(name); err != nil {
			return fmt.Errorf("cli: remove mcp server: %w", err)
		}
	} else {
		if err := mcp.RemoveProjectServer(".", name); err != nil {
			return fmt.Errorf("cli: remove project mcp server: %w", err)
		}
	}

	c.log.Finfo("mcp server %q removed (%s)", name, scope)
	fmt.Fprintf(out, "removed mcp server %q from %s\n", name, scope)
	return nil
}

func (c *CLI) newMCPToolsCommand() *cobra.Command {
	var server string
	cmd := &cobra.Command{
		Use:   "tools",
		Short: "List tools exposed by connected MCP servers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runMCPTools(cmd.OutOrStdout(), server)
		},
	}
	cmd.Flags().StringVar(&server, "server", "", "only list tools for this server")
	return cmd
}

func (c *CLI) runMCPTools(out io.Writer, server string) error {
	if c.mcpMgr != nil {
		c.mcpMgr.Wait()
	}

	type row struct {
		server string
		name   string
	}
	var rows []row
	for _, t := range tools.List() {
		mt, ok := t.(*mcp.MCPTool)
		if !ok {
			continue
		}
		if server != "" && mt.Server() != server {
			continue
		}
		rows = append(rows, row{server: mt.Server(), name: t.Name()})
	}

	if len(rows) == 0 {
		if server != "" {
			fmt.Fprintf(out, "no MCP tools registered for server %q\n", server)
		} else {
			fmt.Fprintln(out, "no MCP tools registered")
		}
		return nil
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].server != rows[j].server {
			return rows[i].server < rows[j].server
		}
		return rows[i].name < rows[j].name
	})
	fmt.Fprintf(out, "%-24s %s\n", "SERVER", "TOOL")
	for _, r := range rows {
		fmt.Fprintf(out, "%-24s %s\n", r.server, r.name)
	}
	return nil
}

func (c *CLI) newMCPSearchCommand() *cobra.Command {
	var limit int
	var asJSON, official bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the Pegg catalog for MCP servers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runMCPSearch(cliContext(cmd), cmd.OutOrStdout(), args[0], limit, official, asJSON)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of results")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.Flags().BoolVar(&official, "official", false, "only show official entries")
	return cmd
}

func (c *CLI) runMCPSearch(ctx context.Context, out io.Writer, query string, limit int, official, asJSON bool) error {
	opts := catalog.SearchOptions{Query: query, Limit: limit}
	if official {
		value := true
		opts.Official = &value
	}
	result, err := catalog.NewClient().Search(ctx, catalog.KindMCP, opts)
	if err != nil {
		return fmt.Errorf("cli: mcp search: %w", err)
	}
	if asJSON {
		data, err := json.MarshalIndent(result.Items, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	if len(result.Items) == 0 {
		fmt.Fprintln(out, "no MCP servers found")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTRANSPORT\tOFFICIAL")
	for _, item := range result.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.ID, item.Name, mcpEntryTransport(item), yesNo(item.Official))
	}
	return w.Flush()
}

func (c *CLI) newMCPInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info <id>",
		Short: "Show MCP server catalog details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runMCPInfo(cliContext(cmd), cmd.OutOrStdout(), args[0])
		},
	}
}

func (c *CLI) runMCPInfo(ctx context.Context, out io.Writer, id string) error {
	mcpID, err := catalog.ParseMCPID(id)
	if err != nil {
		return fmt.Errorf("cli: mcp info: %w", err)
	}
	entry, err := catalog.NewClient().Get(ctx, catalog.KindMCP, mcpID)
	if err != nil {
		return fmt.Errorf("cli: mcp info: %w", err)
	}

	fmt.Fprintf(out, "ID:           %s\n", entry.ID)
	fmt.Fprintf(out, "Name:         %s\n", entry.Name)
	if entry.Description != "" {
		fmt.Fprintf(out, "Description:  %s\n", entry.Description)
	}
	if entry.Vendor != "" {
		fmt.Fprintf(out, "Vendor:       %s\n", entry.Vendor)
	}
	if entry.Version != "" {
		fmt.Fprintf(out, "Version:      %s\n", entry.Version)
	}
	fmt.Fprintf(out, "Official:     %s\n", yesNo(entry.Official))
	fmt.Fprintf(out, "Transport:    %s\n", mcpEntryTransport(entry))
	if entry.Repository != "" {
		fmt.Fprintf(out, "Repository:   %s\n", entry.Repository)
	}
	if entry.Homepage != "" {
		fmt.Fprintf(out, "Homepage:     %s\n", entry.Homepage)
	}
	if entry.License != "" {
		fmt.Fprintf(out, "License:      %s\n", entry.License)
	}
	if len(entry.Tags) > 0 {
		fmt.Fprintf(out, "Tags:         %s\n", strings.Join(entry.Tags, ", "))
	}
	if len(entry.RequiredEnv) > 0 {
		fmt.Fprintln(out, "Required env:")
		for _, key := range entry.RequiredEnv {
			status := "missing"
			if strings.TrimSpace(os.Getenv(key)) != "" {
				status = "set"
			}
			fmt.Fprintf(out, "  %-20s %s\n", key, status)
		}
	}
	fmt.Fprintf(out, "Install:      pegg mcp install %s\n", entry.ID)
	return nil
}

type mcpInstallFlags struct {
	global  bool
	project bool
	name    string
	env     map[string]string
	headers map[string]string
	dryRun  bool
	yes     bool
}

func (c *CLI) newMCPInstallCommand() *cobra.Command {
	f := &mcpInstallFlags{}
	cmd := &cobra.Command{
		Use:   "install <id>",
		Short: "Install an MCP server from the Pegg catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runMCPInstall(cliContext(cmd), cmd, args[0], f)
		},
	}
	cmd.Flags().BoolVar(&f.global, "global", true, "write to the global config")
	cmd.Flags().BoolVar(&f.project, "project", false, "write to the project .mcp.json")
	cmd.MarkFlagsMutuallyExclusive("global", "project")
	cmd.Flags().StringVar(&f.name, "name", "", "server name (default: catalog id)")
	cmd.Flags().StringToStringVar(&f.env, "env", nil, "placeholder value KEY=VALUE (repeatable)")
	cmd.Flags().StringToStringVar(&f.headers, "header", nil, "header override KEY=VALUE (repeatable)")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "print the config without writing")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "install without confirmation")
	return cmd
}

func (c *CLI) runMCPInstall(ctx context.Context, cmd *cobra.Command, rawID string, f *mcpInstallFlags) error {
	out := cmd.OutOrStdout()
	id, err := catalog.ParseMCPID(rawID)
	if err != nil {
		return fmt.Errorf("cli: mcp install: %w", err)
	}
	entry, err := catalog.NewClient().Get(ctx, catalog.KindMCP, id)
	if err != nil {
		return fmt.Errorf("cli: mcp install: %w", err)
	}

	values := make(map[string]string, len(f.env))
	for key, value := range f.env {
		values[key] = value
	}
	for _, key := range entry.RequiredEnv {
		if values[key] == "" {
			if value := os.Getenv(key); value != "" {
				values[key] = value
			}
		}
	}

	server, missing, err := entry.Config(values)
	if err != nil {
		return fmt.Errorf("cli: mcp install: %w", err)
	}
	if !f.dryRun && len(missing) > 0 && isTerminalInput() {
		for _, key := range missing {
			value, err := promptSecret(cmd, key)
			if err != nil {
				return err
			}
			if value != "" {
				values[key] = value
			}
		}
		server, missing, err = entry.Config(values)
		if err != nil {
			return fmt.Errorf("cli: mcp install: %w", err)
		}
	}
	if !f.dryRun && len(missing) > 0 {
		return fmt.Errorf(
			"cli: mcp install: missing required values: %s (set them in the environment or pass --env KEY=VALUE)",
			strings.Join(missing, ", "),
		)
	}

	for key, value := range f.headers {
		if server.URL == "" {
			return errors.New("cli: mcp install: --header only applies to remote MCP servers")
		}
		if server.Headers == nil {
			server.Headers = map[string]string{}
		}
		server.Headers[key] = value
	}

	name := strings.TrimSpace(f.name)
	if name == "" {
		name = entry.ID
	}
	if name == "" {
		name = id
	}
	if name == "" || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("cli: mcp install: invalid server name %q", name)
	}

	scope := mcpScopeGlobal
	if f.project {
		scope = mcpScopeProject
	}

	if f.dryRun {
		payload := map[string]any{}
		redactValues := make(map[string]string, len(values)+len(f.headers))
		for key, value := range values {
			redactValues[key] = value
		}
		for key, value := range f.headers {
			redactValues[key] = value
		}
		redacted := redactMCPServerConfig(server, redactValues)
		if scope == mcpScopeProject {
			payload["mcpServers"] = map[string]config.MCPServerConfig{name: redacted}
		} else {
			payload["mcp_servers"] = map[string]config.MCPServerConfig{name: redacted}
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}

	if !f.yes && isTerminalInput() {
		ok, err := confirm(cmd, fmt.Sprintf("Install MCP server %s (%s)?", name, scope))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(out, "Cancelled.")
			return nil
		}
	}

	switch scope {
	case mcpScopeProject:
		if err := mcp.UpsertProjectServer(".", name, server); err != nil {
			return fmt.Errorf("cli: save project mcp server: %w", err)
		}
	default:
		if err := config.UpsertMCPServer(name, server); err != nil {
			return fmt.Errorf("cli: save mcp server: %w", err)
		}
	}

	c.log.Finfo("mcp server %q installed from catalog (%s)", name, scope)
	fmt.Fprintf(out, "Installed MCP server %q (%s)\n", name, scope)
	fmt.Fprintf(out, "Transport: %s\n", mcpConfigTransport(server))
	if len(entry.RequiredEnv) > 0 {
		fmt.Fprintf(out, "Required env: %s\n", strings.Join(entry.RequiredEnv, ", "))
	}
	return nil
}

func mcpEntryTransport(entry catalog.Entry) string {
	if value := strings.TrimSpace(entry.Transport.Type); value != "" {
		return value
	}
	if len(entry.Transports) > 0 {
		return strings.Join(entry.Transports, ",")
	}
	return "-"
}

func mcpConfigTransport(cfg config.MCPServerConfig) string {
	if value := strings.TrimSpace(cfg.Type); value != "" {
		return value
	}
	if cfg.URL != "" {
		return "sse"
	}
	return "stdio"
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func isTerminalInput() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func promptSecret(cmd *cobra.Command, key string) (string, error) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s: ", key)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		data, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(cmd.OutOrStdout())
		if err != nil {
			return "", fmt.Errorf("cli: read %s: %w", key, err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("cli: read %s: %w", key, err)
	}
	return strings.TrimSpace(line), nil
}

func redactMCPServerConfig(cfg config.MCPServerConfig, values map[string]string) config.MCPServerConfig {
	keys := make([]string, 0, len(values))
	for key, value := range values {
		if value != "" {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(values[keys[i]]) != len(values[keys[j]]) {
			return len(values[keys[i]]) > len(values[keys[j]])
		}
		return keys[i] < keys[j]
	})
	redact := func(value string) string {
		for _, key := range keys {
			value = strings.ReplaceAll(value, values[key], "${"+key+"}")
		}
		return value
	}

	out := cfg
	out.Command = redact(cfg.Command)
	out.URL = redact(cfg.URL)
	if cfg.Args != nil {
		out.Args = make([]string, len(cfg.Args))
		for i, arg := range cfg.Args {
			out.Args[i] = redact(arg)
		}
	}
	if cfg.Env != nil {
		out.Env = make(map[string]string, len(cfg.Env))
		for key, value := range cfg.Env {
			out.Env[key] = redact(value)
		}
	}
	if cfg.Headers != nil {
		out.Headers = make(map[string]string, len(cfg.Headers))
		for key, value := range cfg.Headers {
			out.Headers[key] = redact(value)
		}
	}
	return out
}

func promptScope() (string, error) {
	items := []string{mcpScopeGlobal, mcpScopeProject}
	return promptSelect("Target", items)
}

func promptTransport() (string, error) {
	items := []string{mcpTransportStdio, "sse (legacy)", mcpTransportStreamableHTTP}
	res, err := promptSelect("Transport", items)
	if err != nil {
		return "", err
	}
	switch res {
	case mcpTransportStdio:
		return mcpTransportStdio, nil
	case mcpTransportStreamableHTTP:
		return mcpTransportStreamableHTTP, nil
	default:
		return mcpTransportSSE, nil
	}
}

func promptSelect(label string, items []string) (string, error) {
	p := promptui.Select{
		Label: label,
		Items: items,
		Size:  len(items),
	}
	_, result, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("cli: %s: %w", label, err)
	}
	return result, nil
}

func promptString(label string) (string, error) {
	p := promptui.Prompt{Label: label}
	result, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("cli: %s: %w", label, err)
	}
	return strings.TrimSpace(result), nil
}

func promptArgs() ([]string, error) {
	var args []string
	for {
		a, err := promptString("Argument (empty to finish)")
		if err != nil {
			return nil, err
		}
		if a == "" {
			break
		}
		args = append(args, a)
	}
	return args, nil
}

func promptEnv() (map[string]string, error) {
	env := make(map[string]string)
	for {
		key, err := promptString("Env key (empty to finish)")
		if err != nil {
			return nil, err
		}
		if key == "" {
			break
		}
		value, err := promptString("Value for " + key)
		if err != nil {
			return nil, err
		}
		env[key] = value
	}
	return env, nil
}

func promptHeaders() (map[string]string, error) {
	headers := make(map[string]string)
	items := append(append([]string{}, popularHeaderKeys...), "Custom header", "Done")
	for {
		sel, err := promptSelect("Header (Done to finish)", items)
		if err != nil {
			return nil, err
		}
		if sel == "Done" {
			break
		}
		key := sel
		if sel == "Custom header" {
			key, err = promptString("Header name")
			if err != nil {
				return nil, err
			}
			if key == "" {
				continue
			}
		}
		value, err := promptString("Value for " + key)
		if err != nil {
			return nil, err
		}
		headers[key] = value
	}
	return headers, nil
}
