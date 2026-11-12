package install

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type SelectOptions struct {
	Pattern string
	GOOS    string
	GOARCH  string
	Repo    string
}

type candidate struct {
	asset Asset
	score int
	os    bool
	arch  bool
}

func SelectAsset(release *Release, opts SelectOptions) (Asset, error) {
	if release == nil || len(release.Assets) == 0 {
		return Asset{}, fmt.Errorf("release has no assets")
	}
	pattern := strings.TrimSpace(opts.Pattern)
	repo := strings.ToLower(strings.TrimSpace(opts.Repo))
	osTokens := osAliases(opts.GOOS)
	archTokens := archAliases(opts.GOARCH)

	var candidates []candidate
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if asset.URL == "" {
			continue
		}
		if pattern != "" {
			matched, err := path.Match(pattern, asset.Name)
			if err != nil {
				return Asset{}, fmt.Errorf("invalid asset pattern %q: %w", pattern, err)
			}
			if !matched && asset.Name != pattern {
				continue
			}
		} else if auxiliaryAsset(name) {
			continue
		}

		score := 0
		if pattern != "" && (asset.Name == pattern || strings.HasPrefix(asset.Name, strings.TrimSuffix(pattern, "*"))) {
			score += 1000
		}
		hasOS := hasToken(name, osTokens)
		hasArch := hasToken(name, archTokens)
		if hasOS {
			score += 50
		}
		if hasArch {
			score += 50
		}
		if repo != "" && strings.Contains(name, repo) {
			score += 30
		}
		if strings.HasPrefix(name, "pegg") {
			score += 10
		}
		if isArchiveName(name) {
			score += 5
		} else {
			score += 10
		}
		candidates = append(candidates, candidate{asset: asset, score: score, os: hasOS, arch: hasArch})
	}

	if len(candidates) == 0 {
		return Asset{}, fmt.Errorf("no matching release asset (use --asset; available: %s)", assetNames(release.Assets))
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].asset.Name < candidates[j].asset.Name
	})

	best := candidates[0]
	if pattern == "" && !best.os && !best.arch && len(candidates) > 1 {
		return Asset{}, fmt.Errorf("cannot determine a %s/%s asset (use --asset; available: %s)", opts.GOOS, opts.GOARCH, assetNames(release.Assets))
	}
	return best.asset, nil
}

func auxiliaryAsset(name string) bool {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "checksum") || strings.Contains(lower, "sha256") || strings.Contains(lower, "sha512") {
		return true
	}
	switch strings.ToLower(path.Ext(lower)) {
	case ".sig", ".asc", ".sbom", ".json", ".yaml", ".yml", ".txt", ".md",
		".deb", ".rpm", ".apk", ".msi", ".dmg", ".pkg", ".7z", ".xz", ".bz2", ".zst", ".exe":
		return true
	}
	return false
}

func assetNames(assets []Asset) string {
	names := make([]string, 0, len(assets))
	for _, asset := range assets {
		names = append(names, asset.Name)
	}
	return strings.Join(names, ", ")
}

func osAliases(goos string) []string {
	switch strings.ToLower(goos) {
	case "darwin":
		return []string{"darwin", "macos", "mac", "osx"}
	case "windows":
		return []string{"windows", "win"}
	case "linux":
		return []string{"linux"}
	case "":
		return nil
	default:
		return []string{strings.ToLower(goos)}
	}
}

func archAliases(goarch string) []string {
	switch strings.ToLower(goarch) {
	case "amd64":
		return []string{"amd64", "x86_64", "x86-64", "x64"}
	case "arm64":
		return []string{"arm64", "aarch64"}
	case "386":
		return []string{"386", "i386", "x86"}
	case "arm":
		return []string{"arm", "armv7", "armhf"}
	case "":
		return nil
	default:
		return []string{strings.ToLower(goarch)}
	}
}

func hasToken(name string, tokens []string) bool {
	for _, token := range tokens {
		if token != "" && strings.Contains(name, token) {
			return true
		}
	}
	return false
}

func isArchiveName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".tar")
}
