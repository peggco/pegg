package update

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"

	"github.com/peggco/pegg/internal/core/config"
)

const repoSlug = "peggco/pegg"

var agentAssets = []string{
	`(?i)^pegg_(linux|darwin|windows)_(amd64|arm64)\.(tar\.gz|zip)$`,
}

type Release struct {
	Version string
}

func newAgentUpdater() (*selfupdate.Updater, error) {
	return selfupdate.NewUpdater(selfupdate.Config{Filters: agentAssets})
}

func DetectLatest(ctx context.Context) (*Release, bool, error) {
	updater, err := newAgentUpdater()
	if err != nil {
		return nil, false, fmt.Errorf("create updater: %w", err)
	}
	latest, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))
	if err != nil {
		return nil, false, fmt.Errorf("detect latest: %w", err)
	}
	if !found {
		return nil, false, nil
	}
	return &Release{
		Version: latest.Version(),
	}, true, nil
}

func IsNewerThan(current, latest string) bool {
	cur, err := semver.NewVersion(current)
	if err != nil {
		return false
	}
	lat, err := semver.NewVersion(latest)
	if err != nil {
		return false
	}
	return lat.GreaterThan(cur)
}

func UpdateToLatest(ctx context.Context) error {
	updater, err := newAgentUpdater()
	if err != nil {
		return fmt.Errorf("create updater: %w", err)
	}
	latest, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug(repoSlug))
	if err != nil {
		return fmt.Errorf("detect latest: %w", err)
	}
	if !found {
		return fmt.Errorf("no release found")
	}

	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	if err := updater.UpdateTo(ctx, latest, exe); err != nil {
		return fmt.Errorf("update binary: %w", err)
	}

	return nil
}

func CurrentVersion() string {
	return config.AppVersion
}
