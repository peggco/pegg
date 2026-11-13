package catalog

import (
	"strings"
	"testing"
)

func TestCompatibilityWarnings(t *testing.T) {
	compatible := Entry{
		Kind: KindPlugin,
		Requires: &PluginRequires{
			Pegg:     ">=0.2.4 <0.3.0",
			Protocol: 2,
		},
		Platforms: []string{"linux", "darwin"},
		Arch:      []string{"amd64", "arm64"},
	}
	if warnings := compatible.CompatibilityWarnings("0.2.4", 2); len(warnings) != 0 {
		t.Fatalf("compatible entry warnings = %v", warnings)
	}

	noRequires := Entry{Kind: KindPlugin}
	if warnings := noRequires.CompatibilityWarnings("0.2.4", 2); len(warnings) != 1 || !strings.Contains(warnings[0], "does not declare") {
		t.Fatalf("no requires warnings = %v", warnings)
	}

	protocol := compatible
	protocol.Requires = &PluginRequires{Pegg: ">=0.2.4 <0.3.0", Protocol: 3}
	if warnings := protocol.CompatibilityWarnings("0.2.4", 2); len(warnings) == 0 || !strings.Contains(warnings[0], "protocol 3") {
		t.Fatalf("protocol warnings = %v", warnings)
	}

	pegg := compatible
	pegg.Requires = &PluginRequires{Pegg: ">=0.3.0", Protocol: 2}
	if warnings := pegg.CompatibilityWarnings("0.2.4", 2); len(warnings) == 0 || !strings.Contains(warnings[0], "requires Pegg") {
		t.Fatalf("pegg warnings = %v", warnings)
	}

	platform := compatible
	platform.Platforms = []string{"windows"}
	if warnings := platform.CompatibilityWarnings("0.2.4", 2); len(warnings) == 0 || !strings.Contains(warnings[0], "supported platform") {
		t.Fatalf("platform warnings = %v", warnings)
	}
}

func TestSameVersion(t *testing.T) {
	if !SameVersion("v1.2.3", "1.2.3") {
		t.Fatal("expected versions to match")
	}
	if SameVersion("1.2.3", "1.2.4") {
		t.Fatal("expected versions to differ")
	}
}
