package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestScaffoldPlugin(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	cmd := &cobra.Command{}
	if err := scaffoldPlugin(cmd, pluginNewOptions{
		dir:     dir,
		module:  "example.com/demo",
		name:    "demo",
		pegg:    "github.com/peggco/pegg",
		version: "v0.2.4",
		replace: "/tmp/pegg",
	}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"go.mod",
		"main.go",
		"plugin.go",
		"skill.go",
		"commands.go",
		"README.md",
		".gitignore",
		".github/workflows/release.yml",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	goMod, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
	if !strings.Contains(string(goMod), "module example.com/demo") || !strings.Contains(string(goMod), "replace github.com/peggco/pegg => /tmp/pegg") {
		t.Fatalf("go.mod: %s", goMod)
	}
	pluginGo, _ := os.ReadFile(filepath.Join(dir, "plugin.go"))
	if !strings.Contains(string(pluginGo), "peggplugin.Base") || !strings.Contains(string(pluginGo), "demo_echo") {
		t.Fatalf("plugin.go: %s", pluginGo)
	}
	skillGo, _ := os.ReadFile(filepath.Join(dir, "skill.go"))
	for _, want := range []string{"ToolBundles", `"example"`, "demo_echo", "AllowedTools"} {
		if !strings.Contains(string(skillGo), want) {
			t.Fatalf("skill.go missing %q:\n%s", want, skillGo)
		}
	}
	gitignore, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(gitignore), "/pegg-demo") || !strings.Contains(string(gitignore), "/bin/") {
		t.Fatalf(".gitignore: %s", gitignore)
	}
	workflow, _ := os.ReadFile(filepath.Join(dir, ".github/workflows/release.yml"))
	for _, want := range []string{
		"uses: peggco/pegg/.github/workflows/plugin-release.yml@v0.2.4",
		"binary: pegg-demo",
		"secrets.GITHUB_TOKEN",
		"permissions:",
	} {
		if !strings.Contains(string(workflow), want) {
			t.Fatalf("release.yml missing %q:\n%s", want, workflow)
		}
	}
	for _, unwanted := range []string{"_pegg", "go mod edit", "PEGG_REF", "PEGG_REPOSITORY", "softprops", "dist/"} {
		if strings.Contains(string(workflow), unwanted) {
			t.Fatalf("release.yml still contains %q:\n%s", unwanted, workflow)
		}
	}
	readme, _ := os.ReadFile(filepath.Join(dir, "README.md"))
	for _, want := range []string{"bin/pegg-demo", "git tag v0.1.0"} {
		if !strings.Contains(string(readme), want) {
			t.Fatalf("README.md missing %q", want)
		}
	}
}

func TestPluginBinaryName(t *testing.T) {
	cases := map[string]string{
		"demo":      "pegg-demo",
		"Demo":      "pegg-demo",
		"my plugin": "pegg-my-plugin",
		"usage":     "pegg-usage",
		"  ":        "pegg-plugin",
	}
	for input, want := range cases {
		if got := pluginBinaryName(input); got != want {
			t.Errorf("pluginBinaryName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestGithubRepoSlug(t *testing.T) {
	cases := map[string]string{
		"github.com/peggco/pegg":         "peggco/pegg",
		"https://github.com/peggco/pegg": "peggco/pegg",
		"github.com/peggco/pegg/":        "peggco/pegg",
		"":                               "peggco/pegg",
	}
	for input, want := range cases {
		if got := githubRepoSlug(input); got != want {
			t.Errorf("githubRepoSlug(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPluginReleaseWorkflowContract(t *testing.T) {
	path := filepath.Join("..", "..", ".github", "workflows", "plugin-release.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reusable workflow missing: %v", err)
	}
	for _, want := range []string{
		"workflow_call:",
		"goreleaser/goreleaser-action@v6",
		"go list -m github.com/peggco/pegg",
		"checksums.txt",
		"secrets.token",
	} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("plugin-release.yml missing %q", want)
		}
	}
}
