package install

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
)

func ExpectedSHA256(ctx context.Context, client *Client, release *Release, asset Asset) (string, error) {
	if digest := strings.TrimSpace(asset.Digest); strings.HasPrefix(strings.ToLower(digest), "sha256:") {
		return strings.ToLower(strings.TrimSpace(digest[len("sha256:"):])), nil
	}
	checksumAsset, ok := findChecksumAsset(release, asset)
	if !ok {
		return "", nil
	}
	data, err := client.Download(ctx, checksumAsset.URL, 2<<20)
	if err != nil {
		return "", fmt.Errorf("download checksums: %w", err)
	}
	hash, ok := ParseChecksums(data, asset.Name)
	if !ok && strings.Contains(strings.ToLower(checksumAsset.Name), ".sha256") {
		fields := strings.Fields(string(data))
		if len(fields) == 1 && len(fields[0]) == 64 {
			hash = strings.ToLower(fields[0])
			ok = true
		}
	}
	if !ok {
		return "", fmt.Errorf("checksum file %s has no entry for %s", checksumAsset.Name, asset.Name)
	}
	return hash, nil
}

func findChecksumAsset(release *Release, asset Asset) (Asset, bool) {
	if release == nil {
		return Asset{}, false
	}
	lowerAsset := strings.ToLower(asset.Name)
	perAsset := []string{lowerAsset + ".sha256", lowerAsset + ".sha256.txt", lowerAsset + ".sha256sum"}
	for _, wanted := range perAsset {
		for _, candidate := range release.Assets {
			if strings.ToLower(candidate.Name) == wanted {
				return candidate, true
			}
		}
	}
	for _, candidate := range release.Assets {
		switch strings.ToLower(candidate.Name) {
		case "checksums.txt", "checksums", "sha256sums.txt", "sha256sums":
			return candidate, true
		}
	}
	return Asset{}, false
}

func ParseChecksums(data []byte, filename string) (string, bool) {
	wanted := path.Base(filename)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := strings.ToLower(fields[0])
		name := strings.TrimPrefix(strings.Join(fields[1:], " "), "*")
		if path.Base(name) == wanted || name == filename || path.Base(strings.TrimPrefix(name, "./")) == wanted {
			if len(hash) != 64 {
				continue
			}
			if _, err := hex.DecodeString(hash); err == nil {
				return hash, true
			}
		}
	}
	return "", false
}

func VerifySHA256(data []byte, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return nil
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
