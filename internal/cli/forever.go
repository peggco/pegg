package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/pegglogin"
)

var (
	foreverLogin  = pegglogin.Login
	foreverVerify = pegglogin.Verify
)

type foreverOptions struct {
	token     string
	noBrowser bool
	port      int
	device    string
}

func (c *CLI) newForeverCommand() *cobra.Command {
	opts := &foreverOptions{}
	cmd := &cobra.Command{
		Use:     "login-pegg",
		Aliases: []string{"forever", "pegg-login"},
		Short:   "Log in to Pegg Forever and save the provider",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runForever(cmd.Context(), cmd.OutOrStdout(), opts)
		},
	}
	cmd.Flags().StringVar(&opts.token, "token", "", "Pegg Forever API key (skips the browser login)")
	cmd.Flags().BoolVar(&opts.noBrowser, "no-browser", false, "print the URL instead of opening a browser")
	cmd.Flags().IntVar(&opts.port, "port", 0, "local callback port (0 chooses a free port)")
	cmd.Flags().StringVar(&opts.device, "device", "", "device name shown on the website and in the key name")
	return cmd
}

func (c *CLI) runForever(ctx context.Context, out io.Writer, opts *foreverOptions) error {
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
	return nil
}
