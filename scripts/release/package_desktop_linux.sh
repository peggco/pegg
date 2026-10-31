#!/usr/bin/env bash
set -euo pipefail

version="${1#v}"
arch="${2:-amd64}"
out="${3:-pkg/desktop/dist}"
mkdir -p "$out"

if ! command -v nfpm >/dev/null 2>&1; then
  go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
  export PATH="$(go env GOPATH)/bin:$PATH"
fi

cat > "$out/nfpm.yaml" <<YAML
name: pegg-desktop
arch: ${arch}
platform: linux
version: ${version}
section: utils
priority: optional
maintainer: Pegg <hello@pegg.dev>
description: Native desktop client for the Pegg AI coding agent.
license: FSL-1.1-MIT
homepage: https://pegg.dev
contents:
  - src: pkg/desktop/build/bin/pegg-desktop
    dst: /usr/bin/pegg-desktop
    file_info:
      mode: 0755
  - src: pkg/desktop/assets/pegg.desktop
    dst: /usr/share/applications/pegg.desktop
  - src: pkg/desktop/assets/appicon.png
    dst: /usr/share/icons/hicolor/512x512/apps/pegg.png
YAML

nfpm package --config "$out/nfpm.yaml" --packager deb --target "$out/pegg-desktop_${version}_${arch}.deb"
nfpm package --config "$out/nfpm.yaml" --packager rpm --target "$out/pegg-desktop-${version}.${arch}.rpm"
stage="$(mktemp -d)"
cp pkg/desktop/build/bin/pegg-desktop "$stage/pegg-desktop"
cp pkg/desktop/assets/pegg.desktop "$stage/pegg.desktop"
cp pkg/desktop/assets/appicon.png "$stage/pegg.png"
tar -C "$stage" -czf "$out/pegg-desktop_${version}_linux_${arch}.tar.gz" pegg-desktop pegg.desktop pegg.png
rm -rf "$stage"
