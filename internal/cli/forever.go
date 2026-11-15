package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/pegglogin"
)

var (
	foreverLogin    = pegglogin.Login
	foreverVerify   = pegglogin.Verify
	foreverDefaults = pegglogin.FetchDefaults
)

type foreverOptions struct {
	token         string
	noBrowser     bool
	port          int
	device        string
	applyDefaults bool
	noDefaults    bool
	defaultsURL   string
}

func (c *CLI) newForeverCommand() *cobra.Command {
	opts := &foreverOptions{}
	cmd := &cobra.Command{
		Use:     "login-pegg",
		Aliases: []string{"forever", "pegg-login"},
		Short:   "Log in to Pegg Forever and save the provider",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runForever(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.token, "token", "", "Pegg Forever API key (skips the browser login)")
	cmd.Flags().BoolVar(&opts.noBrowser, "no-browser", false, "print the URL instead of opening a browser")
	cmd.Flags().IntVar(&opts.port, "port", 0, "local callback port (0 chooses a free port)")
	cmd.Flags().StringVar(&opts.device, "device", "", "device name shown on the website and in the key name")
	cmd.Flags().BoolVar(&opts.applyDefaults, "defaults", false, "apply recommended defaults without asking")
	cmd.Flags().BoolVar(&opts.noDefaults, "no-defaults", false, "skip recommended defaults")
	cmd.Flags().StringVar(&opts.defaultsURL, "defaults-url", "", "override the recommended defaults endpoint")
	cmd.MarkFlagsMutuallyExclusive("defaults", "no-defaults")
	return cmd
}

func (c *CLI) runForever(cmd *cobra.Command, opts *foreverOptions) error {
	ctx := cliContext(cmd)
	out := cmd.OutOrStdout()
	if out == nil {
		out = io.Discard
	}

	var cfg *pegglogin.Config
	if token := strings.TrimSpace(opts.token); token != "" {
		cfg = &pegglogin.Config{
			Name:    pegglogin.ProviderName,
			Driver:  "openai",
			BaseURL: pegglogin.DefaultBaseURL,
			APIKey:  token,
			Timeout: pegglogin.DefaultTimeout,
		}
	} else {
		loginCfg, err := foreverLogin(ctx, pegglogin.Options{
			Device:    strings.TrimSpace(opts.device),
			Port:      opts.port,
			NoBrowser: opts.noBrowser,
			OnURL: func(authorizeURL string) {
				fmt.Fprintf(out, "Open this URL to log in to Pegg:\n%s\n\n", authorizeURL)
				fmt.Fprintln(out, "Waiting for browser approval...")
			},
		})
		if err != nil {
			return fmt.Errorf("cli: pegg login: %w", err)
		}
		cfg = loginCfg
	}

	if err := foreverVerify(ctx, cfg.BaseURL, cfg.APIKey, nil); err != nil {
		fmt.Fprintf(out, "Warning: could not verify the Pegg provider: %v; saving anyway\n", err)
	}

	if err := pegglogin.Apply(cfg); err != nil {
		return fmt.Errorf("cli: save Pegg provider: %w", err)
	}

	saved := config.LLMConfig{
		Provider: pegglogin.ProviderName,
		Driver:   cfg.Driver,
		BaseURL:  cfg.BaseURL,
		APIKey:   cfg.APIKey,
		Timeout:  cfg.Timeout,
	}
	if err := c.syncProvider(saved); err != nil {
		fmt.Fprintf(out, "Warning: Pegg provider saved but models could not be synced: %v\n", err)
	}

	providerName := cfg.Name
	if providerName == "" {
		providerName = pegglogin.ProviderName
	}
	fmt.Fprintf(out, "Logged in to Pegg\nprovider: %s\nbase URL: %s\n", providerName, cfg.BaseURL)
	return c.applyRecommendedDefaults(cmd, out, opts)
}

func (c *CLI) applyRecommendedDefaults(cmd *cobra.Command, out io.Writer, opts *foreverOptions) error {
	if opts.noDefaults {
		fmt.Fprintln(out, "Skipped recommended defaults.")
		return nil
	}
	peggCfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(out, "Warning: could not read config for recommended defaults: %v\n", err)
		return nil
	}
	d, err := foreverDefaults(cliContext(cmd), pegglogin.DefaultsEndpoint(opts.defaultsURL), nil)
	if err != nil {
		fmt.Fprintf(out, "Warning: could not load recommended defaults: %v\n", err)
		return nil
	}
	report := pegglogin.ApplyDefaults(peggCfg, d, pegglogin.ProviderName)
	if report.Empty() {
		return nil
	}
	if len(report.Changes) > 0 {
		fmt.Fprintln(out, "Recommended defaults:")
		for _, change := range report.Changes {
			fmt.Fprintf(out, "  %s\n", change)
		}
	}
	if len(report.Skipped) > 0 {
		fmt.Fprintln(out, "Skipped:")
		for _, skipped := range report.Skipped {
			fmt.Fprintf(out, "  %s\n", skipped)
		}
	}
	apply := opts.applyDefaults
	if !apply {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintln(out, "Run 'pegg login-pegg --defaults' to apply them.")
			return nil
		}
		fmt.Fprint(out, "Apply these defaults? [Y/n] ")
		line, readErr := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if readErr != nil && strings.TrimSpace(line) == "" {
			return nil
		}
		answer := strings.ToLower(strings.TrimSpace(line))
		apply = answer == "" || answer == "y" || answer == "yes"
	}
	if !apply {
		return nil
	}
	if err := config.Update(func(cfg *config.Config) error {
		pegglogin.ApplyDefaults(cfg, d, pegglogin.ProviderName)
		return nil
	}); err != nil {
		fmt.Fprintf(out, "Warning: could not apply recommended defaults: %v\n", err)
		return nil
	}
	fmt.Fprintln(out, "Applied recommended defaults.")
	return nil
}
