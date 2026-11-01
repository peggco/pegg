package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/peggco/pegg/internal/skillhub"
)

func writeTestSkill(t *testing.T, dir, name string) {
	t.Helper()
	target := filepath.Join(dir, name)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: test skill\n---\n\nDo the thing.\n"
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseSkillScopeChoice(t *testing.T) {
	if parseSkillScopeChoice("2\n") != true {
		t.Fatal("choice 2 should be project")
	}
	if parseSkillScopeChoice("") != false {
		t.Fatal("default should be global")
	}
	if parseSkillScopeChoice("1") != false {
		t.Fatal("choice 1 should be global")
	}
}

func TestRunSkillsList(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(project)

	writeTestSkill(t, filepath.Join(home, ".agents", "skills"), "agents-skill")
	writeTestSkill(t, filepath.Join(home, ".pegg", "skills"), "global-skill")
	writeTestSkill(t, filepath.Join(project, ".pegg", "skills"), "project-skill")

	c := &CLI{}
	var out bytes.Buffer
	if err := c.runSkillsList(&out, false, false, true); err != nil {
		t.Fatal(err)
	}
	var items []skillhub.Installed
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		t.Fatalf("json: %v\n%s", err, out.String())
	}
	if len(items) != 3 {
		t.Fatalf("items = %+v", items)
	}
	scopes := map[string]string{}
	for _, item := range items {
		scopes[item.Name] = item.Scope
	}
	if scopes["agents-skill"] != "agents" || scopes["global-skill"] != "global" || scopes["project-skill"] != "project" {
		t.Fatalf("scopes = %+v", scopes)
	}
}
