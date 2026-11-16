package plugin

import (
	"strings"
	"testing"
)

func TestPluginCmdAppliesConfiguredEnv(t *testing.T) {
	t.Setenv("PEGG_TEST_INHERITED", "1")

	cmd := pluginCmd("plugin", map[string]string{"B": "2", "A": "1"})

	joined := strings.Join(cmd.Env, "\n")
	if !strings.Contains(joined, "PEGG_TEST_INHERITED=1") {
		t.Fatalf("process env was not inherited: %#v", cmd.Env)
	}
	if len(cmd.Env) < 2 {
		t.Fatalf("env = %#v", cmd.Env)
	}
	last := cmd.Env[len(cmd.Env)-2:]
	if last[0] != "A=1" || last[1] != "B=2" {
		t.Fatalf("configured env = %#v, want sorted A then B", last)
	}
}

func TestPluginCmdWithoutEnvLeavesEnvUnset(t *testing.T) {
	cmd := pluginCmd("plugin", nil)
	if cmd.Env != nil {
		t.Fatalf("env = %#v, want nil", cmd.Env)
	}
}
