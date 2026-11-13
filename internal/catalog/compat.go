package catalog

import (
	"fmt"
	"runtime"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
)

func (e Entry) CompatibilityWarnings(peggVersion string, protocolVersion int) []string {
	if e.Kind != "" && e.Kind != KindPlugin {
		return nil
	}

	var warnings []string
	if e.Requires == nil {
		return []string{
			"plugin does not declare Pegg compatibility (requires.pegg / requires.protocol); installing may break",
		}
	}

	if e.Requires.Protocol == 0 {
		warnings = append(warnings,
			"plugin does not declare a protocol version; the runtime handshake will decide")
	} else if e.Requires.Protocol != protocolVersion {
		warnings = append(warnings, fmt.Sprintf(
			"plugin protocol %d does not match Pegg protocol %d; not recommended, the plugin may fail to load",
			e.Requires.Protocol, protocolVersion))
	}

	if strings.TrimSpace(e.Requires.Pegg) == "" {
		warnings = append(warnings,
			"plugin does not declare a supported Pegg version; installing may break")
	} else if constraint, err := semver.NewConstraint(e.Requires.Pegg); err != nil {
		warnings = append(warnings, fmt.Sprintf(
			"plugin Pegg requirement %q is invalid; not recommended", e.Requires.Pegg))
	} else if version, err := semver.NewVersion(strings.TrimPrefix(strings.TrimSpace(peggVersion), "v")); err != nil {
		warnings = append(warnings, fmt.Sprintf(
			"cannot parse current Pegg version %q; compatibility not checked", peggVersion))
	} else if !constraint.Check(version) {
		warnings = append(warnings, fmt.Sprintf(
			"plugin requires Pegg %s, current version is %s; not recommended, installing may break",
			e.Requires.Pegg, peggVersion))
	}

	if len(e.Platforms) > 0 && !slices.Contains(e.Platforms, runtime.GOOS) {
		warnings = append(warnings, fmt.Sprintf(
			"plugin does not list %s as a supported platform; installation may fail", runtime.GOOS))
	}
	if len(e.Arch) > 0 && !slices.Contains(e.Arch, runtime.GOARCH) {
		warnings = append(warnings, fmt.Sprintf(
			"plugin does not list %s as a supported architecture; installation may fail", runtime.GOARCH))
	}

	return warnings
}

func SameVersion(a, b string) bool {
	normalize := func(value string) string {
		return strings.TrimPrefix(strings.TrimSpace(value), "v")
	}
	return normalize(a) == normalize(b)
}
