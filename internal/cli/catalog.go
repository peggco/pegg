package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type catalogPublishFlags struct {
	kind   string
	file   string
	repo   string
	branch string
}

type catalogPublishTransport struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type catalogPublishEntry struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Kind          string                  `json:"kind"`
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description"`
	SkillID       string                  `json:"skillId"`
	Source        string                  `json:"source"`
	Repository    string                  `json:"repository"`
	Transport     catalogPublishTransport `json:"transport"`
	RequiredEnv   []string                `json:"requiredEnv"`
}

var (
	catalogMCPID     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	catalogRepo      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	catalogSkillID   = regexp.MustCompile(`^[^/\\\s]+$`)
	catalogSkillSrc  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(?:/[A-Za-z0-9][A-Za-z0-9._-]*)?$`)
	catalogPlacehold = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)\}`)
)

func (c *CLI) newCatalogCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Publish entries to the Pegg Hub catalog",
	}
	cmd.AddCommand(c.newCatalogPublishCommand())
	return cmd
}

func (c *CLI) newCatalogPublishCommand() *cobra.Command {
	f := &catalogPublishFlags{}
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Open a pull request against peggco/hub-registry",
		Long: "Validate a catalog entry locally and print a GitHub new-file URL.\n" +
			"Open the URL to submit the entry as a pull request. No hub credentials are needed.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return c.runCatalogPublish(cmd, f)
		},
	}
	cmd.Flags().StringVar(&f.kind, "kind", "", "entry kind: mcp, plugin, or skill (default: from the file)")
	cmd.Flags().StringVar(&f.file, "file", "-", "JSON entry file, or - for stdin")
	cmd.Flags().StringVar(&f.repo, "repo", "peggco/hub-registry", "target GitHub repository")
	cmd.Flags().StringVar(&f.branch, "branch", "main", "target branch")
	return cmd
}

func (c *CLI) runCatalogPublish(cmd *cobra.Command, f *catalogPublishFlags) error {
	data, err := readCatalogPublishInput(f.file, cmd.InOrStdin())
	if err != nil {
		return fmt.Errorf("cli: catalog publish: %w", err)
	}

	var entry catalogPublishEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return fmt.Errorf("cli: catalog publish: parse JSON: %w", err)
	}

	kind := strings.ToLower(strings.TrimSpace(f.kind))
	if kind == "" {
		kind = strings.ToLower(strings.TrimSpace(entry.Kind))
	}
	if entry.Kind != "" && kind != strings.ToLower(strings.TrimSpace(entry.Kind)) {
		return fmt.Errorf("cli: catalog publish: --kind %q does not match entry kind %q", kind, entry.Kind)
	}
	if err := validateCatalogPublish(kind, entry); err != nil {
		return fmt.Errorf("cli: catalog publish: %w", err)
	}

	target, err := catalogTargetPath(kind, entry)
	if err != nil {
		return fmt.Errorf("cli: catalog publish: %w", err)
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, data, "", "  "); err != nil {
		return fmt.Errorf("cli: catalog publish: format JSON: %w", err)
	}
	pretty.WriteByte('\n')

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Target: %s\n", target)
	fmt.Fprintln(out, "Open this URL to submit the entry as a pull request:")
	fmt.Fprintln(out, githubNewFileURL(f.repo, f.branch, target, pretty.Bytes()))
	return nil
}

func readCatalogPublishInput(file string, stdin io.Reader) ([]byte, error) {
	if strings.TrimSpace(file) == "" || file == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(file)
}

func validateCatalogPublish(kind string, entry catalogPublishEntry) error {
	if entry.SchemaVersion != 1 {
		return errors.New("schemaVersion must be 1")
	}
	if strings.TrimSpace(entry.Name) == "" {
		return errors.New("name is required")
	}
	description := strings.TrimSpace(entry.Description)
	if len(description) < 20 || len(description) > 280 {
		return errors.New("description must be 20-280 characters")
	}

	switch kind {
	case "mcp":
		if !catalogMCPID.MatchString(strings.TrimSpace(entry.ID)) {
			return errors.New("mcp id must match ^[a-z0-9][a-z0-9-]{0,63}$")
		}
		if err := validateCatalogTransport(entry); err != nil {
			return err
		}
	case "plugin":
		repository := strings.TrimSpace(entry.Repository)
		if !catalogRepo.MatchString(repository) {
			return errors.New("repository must be owner/repo")
		}
		if strings.TrimSpace(entry.ID) != repository {
			return errors.New("plugin id must equal repository")
		}
	case "skill":
		if !catalogSkillSrc.MatchString(strings.TrimSpace(entry.Source)) {
			return errors.New("source must be owner/repo or a host")
		}
		if !catalogSkillID.MatchString(strings.TrimSpace(entry.SkillID)) {
			return errors.New("skillId must not contain spaces or slashes")
		}
	default:
		return fmt.Errorf("unsupported kind %q", kind)
	}
	return nil
}

func validateCatalogTransport(entry catalogPublishEntry) error {
	transport := strings.ToLower(strings.TrimSpace(entry.Transport.Type))
	switch transport {
	case "stdio", "local":
		if strings.TrimSpace(entry.Transport.Command) == "" {
			return errors.New("stdio transport requires command")
		}
	case "sse", "http", "streamable", "streamable-http":
		if strings.TrimSpace(entry.Transport.URL) == "" {
			return errors.New("remote transport requires url")
		}
	default:
		return fmt.Errorf("unsupported transport %q", entry.Transport.Type)
	}

	required := make(map[string]bool, len(entry.RequiredEnv))
	for _, key := range entry.RequiredEnv {
		required[key] = true
	}
	for _, value := range []string{
		entry.Transport.Command,
		entry.Transport.URL,
		strings.Join(entry.Transport.Args, " "),
		strings.Join(mapValues(entry.Transport.Env), " "),
		strings.Join(mapValues(entry.Transport.Headers), " "),
	} {
		for _, match := range catalogPlacehold.FindAllStringSubmatch(value, -1) {
			if !required[match[1]] {
				return fmt.Errorf("placeholder ${%s} is missing from requiredEnv", match[1])
			}
		}
	}
	return nil
}

func catalogTargetPath(kind string, entry catalogPublishEntry) (string, error) {
	switch kind {
	case "mcp":
		return "catalog/mcp/" + strings.TrimSpace(entry.ID) + ".json", nil
	case "plugin":
		return "catalog/plugins/" + strings.TrimSpace(entry.Repository) + ".json", nil
	case "skill":
		return "catalog/skills/" + strings.TrimSpace(entry.Source) + "/" + strings.TrimSpace(entry.SkillID) + ".json", nil
	default:
		return "", fmt.Errorf("unsupported kind %q", kind)
	}
}

func githubNewFileURL(repo, branch, target string, content []byte) string {
	repo = strings.Trim(strings.TrimSpace(repo), "/")
	branch = strings.Trim(strings.TrimSpace(branch), "/")
	target = strings.TrimPrefix(target, "/")
	base := fmt.Sprintf("https://github.com/%s/new/%s/%s", repo, branch, target)
	q := url.Values{}
	q.Set("filename", path.Base(target))
	q.Set("value", string(content))
	return base + "?" + q.Encode()
}

func mapValues(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}
