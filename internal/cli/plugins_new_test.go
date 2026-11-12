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
	for _, name := range []string{"go.mod", "main.go", "plugin.go", "commands.go", "README.md"} {
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
}
