package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/peggco/pegg/internal/plugin"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type hookManager interface {
	HookSpecs() []plugin.PluginHookSpec
	ApplyHook(context.Context, string, pkgplugin.HookKind, json.RawMessage, time.Duration) (plugin.HookOutcome, error)
	Close()
}

type hookListEntry struct {
	Plugin      string `json:"plugin"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Priority    int    `json:"priority"`
	Description string `json:"description,omitempty"`
}

func (c *CLI) newHooksCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Inspect and run plugin hooks",
	}
	cmd.AddCommand(
		c.newHooksListCommand(),
		c.newHooksApplyCommand(),
		c.newHooksServeCommand(),
	)
	return cmd
}

func (c *CLI) newHooksListCommand() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered plugin hooks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = os.Setenv("PEGG_HOOKS_SERVE", "1")
			mgr, err := c.hooksManager()
			if err != nil {
				return err
			}
			defer mgr.Close()

			specs := mgr.HookSpecs()
			entries := make([]hookListEntry, 0, len(specs))
			for _, spec := range specs {
				entries = append(entries, hookListEntry{
					Plugin:      spec.Plugin,
					Name:        spec.Name,
					Kind:        string(spec.Kind),
					Priority:    spec.Priority,
					Description: spec.Description,
				})
			}
			if asJSON {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(entries)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hooks registered.")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "PLUGIN\tNAME\tKIND\tPRIORITY\tDESCRIPTION")
			fmt.Fprintln(w, "------\t----\t----\t--------\t-----------")
			for _, entry := range entries {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", entry.Plugin, entry.Name, entry.Kind, entry.Priority, entry.Description)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: %d hooks\n", len(entries))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func (c *CLI) newHooksApplyCommand() *cobra.Command {
	var kind string
	var value string
	var timeout time.Duration
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "apply <name>",
		Short: "Run all handlers registered for a hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("hook name is required")
			}
			if timeout <= 0 {
				return fmt.Errorf("timeout must be positive")
			}
			mgr, err := c.hooksManager()
			if err != nil {
				return err
			}
			defer mgr.Close()

			hookKind, err := resolveHookKind(mgr.HookSpecs(), name, kind)
			if err != nil {
				return err
			}
			raw, err := readHookValue(cmd, value)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			applyCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			outcome, err := mgr.ApplyHook(applyCtx, name, hookKind, raw, timeout)
			if err != nil {
				return err
			}
			return printHookOutcome(cmd.OutOrStdout(), outcome, asJSON)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "hook kind: filter or action")
	cmd.Flags().StringVar(&value, "value", "", "JSON payload, or - to read from stdin")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "handler timeout")
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

type hookServeRequest struct {
	Name      string          `json:"name"`
	Kind      string          `json:"kind"`
	Value     json.RawMessage `json:"value,omitempty"`
	TimeoutMS int64           `json:"timeout_ms,omitempty"`
}

type hookServeResponse struct {
	Value   json.RawMessage   `json:"value,omitempty"`
	Outputs []json.RawMessage `json:"outputs,omitempty"`
	Errors  []string          `json:"errors,omitempty"`
	Error   string            `json:"error,omitempty"`
}

func (c *CLI) newHooksServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run a long-lived hook dispatcher over newline-delimited JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = os.Setenv("PEGG_HOOKS_SERVE", "1")
			mgr, err := c.hooksManager()
			if err != nil {
				return err
			}
			defer mgr.Close()

			signals := make(chan os.Signal, 1)
			signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(signals)
			go func() {
				<-signals
				mgr.Close()
				os.Exit(0)
			}()

			decoder := json.NewDecoder(cmd.InOrStdin())
			encoder := json.NewEncoder(cmd.OutOrStdout())
			for {
				var req hookServeRequest
				if err := decoder.Decode(&req); err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}
				kind := pkgplugin.HookKind(req.Kind)
				if kind == "" {
					kind = pkgplugin.HookFilter
				}
				timeout := time.Duration(req.TimeoutMS) * time.Millisecond
				if timeout <= 0 {
					timeout = 30 * time.Second
				}
				outcome, applyErr := mgr.ApplyHook(cmd.Context(), req.Name, kind, req.Value, timeout)
				resp := hookServeResponse{
					Value:   outcome.Value,
					Outputs: outcome.Outputs,
					Errors:  outcome.Errors,
				}
				if applyErr != nil {
					resp.Error = applyErr.Error()
				}
				if err := encoder.Encode(resp); err != nil {
					return err
				}
			}
		},
	}
}

func (c *CLI) hooksManager() (hookManager, error) {
	if c.hooksBackend != nil {
		return c.hooksBackend()
	}
	if c.pluginMgr != nil {
		return c.pluginMgr, nil
	}
	mgr := plugin.NewManager(c.log, c.cfg, c.bus, c.cache, c.llmMgr, c.sessions, c.fs, c.mcpMgr, c.lspMgr)
	if c.cfg != nil {
		mgr.SetExcludedPlugins(c.cfg.Plugins.Exclude)
	}
	if err := mgr.LoadPlugins(); err != nil {
		mgr.Close()
		return nil, fmt.Errorf("load plugins: %w", err)
	}
	return mgr, nil
}

func resolveHookKind(specs []plugin.PluginHookSpec, name, requested string) (pkgplugin.HookKind, error) {
	switch strings.ToLower(strings.TrimSpace(requested)) {
	case string(pkgplugin.HookFilter):
		return pkgplugin.HookFilter, nil
	case string(pkgplugin.HookAction):
		return pkgplugin.HookAction, nil
	case "":
	default:
		return "", fmt.Errorf("invalid --kind %q: use filter or action", requested)
	}

	kinds := make(map[pkgplugin.HookKind]bool)
	for _, spec := range specs {
		if spec.Name == name {
			kinds[spec.Kind] = true
		}
	}
	if len(kinds) == 0 {
		return "", fmt.Errorf("hook %q: no handler registered", name)
	}
	if len(kinds) > 1 {
		return "", fmt.Errorf("hook %q has both filter and action handlers; specify --kind", name)
	}
	for specKind := range kinds {
		return specKind, nil
	}
	return "", fmt.Errorf("hook %q: no handler registered", name)
}

func readHookValue(cmd *cobra.Command, value string) (json.RawMessage, error) {
	raw := value
	if raw == "-" || (raw == "" && !isTerminal(cmd.InOrStdin())) {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, fmt.Errorf("read hook value from stdin: %w", err)
		}
		raw = string(data)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "null"
	}
	if !json.Valid([]byte(raw)) {
		return nil, fmt.Errorf("invalid hook value: expected JSON")
	}
	return json.RawMessage(raw), nil
}

func printHookOutcome(w io.Writer, outcome plugin.HookOutcome, asJSON bool) error {
	outputs := outcome.Outputs
	if outputs == nil {
		outputs = []json.RawMessage{}
	}
	errs := outcome.Errors
	if errs == nil {
		errs = []string{}
	}
	if asJSON {
		payload := struct {
			Value   json.RawMessage   `json:"value"`
			Outputs []json.RawMessage `json:"outputs"`
			Errors  []string          `json:"errors"`
		}{Value: outcome.Value, Outputs: outputs, Errors: errs}
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	}

	printed := false
	if len(outcome.Value) > 0 && string(outcome.Value) != "null" {
		fmt.Fprintln(w, string(outcome.Value))
		printed = true
	}
	for _, output := range outputs {
		fmt.Fprintln(w, string(output))
		printed = true
	}
	for _, message := range errs {
		fmt.Fprintln(w, "error: "+message)
		printed = true
	}
	if !printed {
		fmt.Fprintln(w, "ok")
	}
	return nil
}
