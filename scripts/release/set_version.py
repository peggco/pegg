from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
VERSION_RE = re.compile(r"^\d+\.\d+\.\d+(?:-[0-9A-Za-z.]+)?$")
PACKAGE_DIRS = ("pkg/vscode", "pkg/chat-ui", "pkg/intellij", "pkg/desktop/frontend")


class Sync:
    def __init__(self, version: str) -> None:
        self.version = version
        self.touched: set[Path] = set()

    def path(self, relative: str) -> Path:
        return ROOT / relative

    def replace_once(
        self,
        relative: str,
        pattern: str,
        replacement,
        *,
        required: bool = True,
    ) -> None:
        path = self.path(relative)
        text = path.read_text()
        updated, count = re.subn(pattern, replacement, text, count=1, flags=re.MULTILINE)
        if count == 0:
            if required:
                raise SystemExit(f"Pattern not found in {relative}: {pattern}")
            print(f"warning: pattern not found in {relative}")
            return
        if updated != text:
            path.write_text(updated)
            self.touched.add(path)

    def bump_changelog(self, relative: str, marker: str, heading: str, existing_pattern: str) -> None:
        path = self.path(relative)
        if not path.exists():
            raise SystemExit(f"Missing changelog: {relative}")
        text = path.read_text()
        if re.search(existing_pattern, text, re.MULTILINE):
            self.touched.add(path)
            return
        if marker in text:
            text = text.replace(marker, f"{marker}\n\n{heading}", 1)
        else:
            lines = text.splitlines()
            if lines and lines[0].startswith("# "):
                text = lines[0] + f"\n\n{marker}\n\n{heading}\n" + "\n".join(lines[1:])
            else:
                text = f"{marker}\n\n{heading}\n\n" + text
        path.write_text(text)
        self.touched.add(path)

    def apply(self) -> None:
        self.replace_once(
            "internal/core/config/constants.go",
            r'(AppVersion\s*=\s*)"[^"]*"',
            lambda m: f'{m.group(1)}"{self.version}"',
        )

        self.replace_once(
            "pkg/intellij/gradle.properties",
            r"^pluginVersion=.*$",
            f"pluginVersion={self.version}",
        )

        fallbacks = (
            (
                "pkg/intellij/src/main/kotlin/dev/pegg/intellij/chat/ChatController.kt",
                r'(\?\.version \?: ")[^"]*(")',
            ),
            (
                "pkg/intellij/src/main/kotlin/dev/pegg/intellij/binary/BinaryManager.kt",
                r'(\?\.version \?: ")[^"]*(")',
            ),
            (
                "pkg/vscode/src/acp/client.ts",
                r"(version: this\.options\.version \?\? ')[^']*(')",
            ),
            (
                "pkg/vscode/src/chat/controller.ts",
                r"(packageJSON\.version \?\? ')[^']*(')",
            ),
        )
        for relative, pattern in fallbacks:
            self.replace_once(
                relative,
                pattern,
                lambda m: f"{m.group(1)}{self.version}{m.group(2)}",
                required=False,
            )

        self.bump_changelog(
            "pkg/vscode/CHANGELOG.md",
            "## Unreleased",
            f"## {self.version}",
            rf"^## {re.escape(self.version)}\s*$",
        )

        today = datetime.now(timezone.utc).strftime("%Y-%m-%d")
        self.bump_changelog(
            "pkg/intellij/CHANGELOG.md",
            "## [Unreleased]",
            f"## [{self.version}] - {today}",
            rf"^## \[{re.escape(self.version)}\]",
        )

        """
        docs = (
            ("pkg/intellij/README.md", r"tag such as `v[^`]+`", f"tag such as `v{self.version}`"),
            ("pkg/vscode/README.md", r"tag such as `v[^`]+`", f"tag such as `v{self.version}`"),
            ("docs/pegg/usage/cli.md", r"^Latest version: .*$", f"Latest version: {self.version}"),
            (
                "docs/pegg/usage/cli.md",
                r"^Updating to version .*$",
                f"Updating to version {self.version}...",
            ),
            (
                "docs/pegg/usage/cli.md",
                r"^Successfully updated to version .*$",
                f"Successfully updated to version {self.version}",
            ),
            ("docs/pegg/usage/acp.md", r'"version": "[^"]+"', f'"version": "{self.version}"'),
        )
        for relative, pattern, replacement in docs:
            self.replace_once(relative, pattern, replacement, required=False)
        """

        npm = shutil.which("npm")
        if not npm:
            raise SystemExit("npm is required to update package-lock.json files")
        for directory in PACKAGE_DIRS:
            self.run_quiet(
                [
                    npm,
                    "version",
                    self.version,
                    "--no-git-tag-version",
                    "--allow-same-version",
                    "--force",
                ],
                cwd=self.path(directory),
            )
            self.touched.add(self.path(directory) / "package.json")
            self.touched.add(self.path(directory) / "package-lock.json")

        self.run_quiet([npm, "pkg", "set", f"version={self.version}"], cwd=self.path("npm"))
        self.touched.add(self.path("npm/package.json"))

    def check(self) -> None:
        errors: list[str] = []

        def expect(name: str, actual: str | None) -> None:
            if actual != self.version:
                errors.append(f"{name}: expected {self.version}, found {actual or 'missing'}")

        constants = self.path("internal/core/config/constants.go").read_text()
        match = re.search(r'AppVersion\s*=\s*"([^"]+)"', constants)
        expect("internal/core/config/constants.go AppVersion", match.group(1) if match else None)

        gradle = self.path("pkg/intellij/gradle.properties").read_text()
        match = re.search(r"^pluginVersion=(.+)$", gradle, re.MULTILINE)
        expect("pkg/intellij/gradle.properties pluginVersion", match.group(1).strip() if match else None)

        for directory in PACKAGE_DIRS:
            expect(f"{directory}/package.json", self.package_version(self.path(directory) / "package.json"))
            lock = self.path(directory) / "package-lock.json"
            if lock.exists():
                data = json.loads(lock.read_text())
                expect(f"{directory}/package-lock.json version", str(data.get("version", "")))
                expect(
                    f"{directory}/package-lock.json packages[''].version",
                    str(data.get("packages", {}).get("", {}).get("version", "")),
                )

        expect("npm/package.json", self.package_version(self.path("npm/package.json")))

        vscode_changelog = self.path("pkg/vscode/CHANGELOG.md").read_text()
        if not re.search(rf"^## {re.escape(self.version)}\s*$", vscode_changelog, re.MULTILINE):
            errors.append(f"pkg/vscode/CHANGELOG.md: missing ## {self.version}")

        intellij_changelog = self.path("pkg/intellij/CHANGELOG.md").read_text()
        if not re.search(rf"^## \[{re.escape(self.version)}\]", intellij_changelog, re.MULTILINE):
            errors.append(f"pkg/intellij/CHANGELOG.md: missing ## [{self.version}]")

        if errors:
            print(f"Version files do not match v{self.version}:", file=sys.stderr)
            for error in errors:
                print(f"  - {error}", file=sys.stderr)
            print(
                f"\nRun: python3 scripts/release/set_version.py {self.version} --commit --tag",
                file=sys.stderr,
            )
            raise SystemExit(1)

        print(f"Version files match v{self.version}.")

    @staticmethod
    def package_version(path: Path) -> str | None:
        try:
            return str(json.loads(path.read_text()).get("version", "")) or None
        except (OSError, json.JSONDecodeError):
            return None

    @staticmethod
    def run_quiet(command: list[str], cwd: Path) -> None:
        result = subprocess.run(command, cwd=cwd, text=True, capture_output=True)
        if result.returncode != 0:
            print(result.stdout, file=sys.stderr)
            print(result.stderr, file=sys.stderr)
            raise SystemExit(f"Command failed: {' '.join(command)}")

    def commit(self) -> None:
        paths = sorted(str(path.relative_to(ROOT)) for path in self.touched)
        if not paths:
            print("No version files changed.")
            return
        self.git("add", "--", *paths)
        changed = subprocess.run(
            ["git", "diff", "--cached", "--quiet", "--", *paths],
            cwd=ROOT,
        )
        if changed.returncode == 0:
            print("No version changes to commit.")
            return
        self.git("commit", "--only", "-m", f"chore: release v{self.version}", "--", *paths)

    def tag(self) -> None:
        tag = f"v{self.version}"
        exists = subprocess.run(
            ["git", "rev-parse", "--verify", "--quiet", f"refs/tags/{tag}"],
            cwd=ROOT,
            text=True,
            capture_output=True,
        )
        if exists.returncode == 0:
            tag_commit = exists.stdout.strip()
            head = subprocess.run(
                ["git", "rev-parse", "HEAD"],
                cwd=ROOT,
                text=True,
                capture_output=True,
            ).stdout.strip()
            if tag_commit != head:
                raise SystemExit(
                    f"Tag {tag} points to {tag_commit}, but HEAD is {head}. "
                    "Delete or move the tag, or choose a new version."
                )
            print(f"Tag {tag} already points to HEAD; leaving it unchanged.")
            return
        self.git("tag", tag)

    def push(self) -> None:
        self.git("push", "origin", "HEAD")
        tag = f"v{self.version}"
        if subprocess.run(
            ["git", "rev-parse", "--verify", "--quiet", f"refs/tags/{tag}"],
            cwd=ROOT,
            text=True,
            capture_output=True,
        ).returncode == 0:
            self.git("push", "origin", tag)
        else:
            print(f"Tag {tag} does not exist; pushed HEAD only.")

    @staticmethod
    def git(*args: str) -> None:
        result = subprocess.run(["git", *args], cwd=ROOT)
        if result.returncode != 0:
            raise SystemExit(f"git {' '.join(args)} failed")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Sync Pegg release versions, changelogs, and docs.",
    )
    parser.add_argument(
        "version",
        nargs="?",
        help="Release version without the v prefix (for example 0.2.1).",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="Only verify that committed versions match the release version.",
    )
    parser.add_argument(
        "--commit",
        action="store_true",
        help="Commit only the files changed by this script.",
    )
    parser.add_argument(
        "--tag",
        action="store_true",
        help="Create the v<version> tag (if it does not already exist).",
    )
    parser.add_argument(
        "--push",
        action="store_true",
        help="Push HEAD and the release tag to origin.",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    version = args.version or os.environ.get("GITHUB_REF_NAME", "")
    version = version[1:] if version.startswith("v") else version

    if not version:
        raise SystemExit("Usage: python3 scripts/release/set_version.py <version>")
    if not VERSION_RE.match(version):
        raise SystemExit(f"Invalid version: {version}")

    sync = Sync(version)
    if args.check:
        sync.check()
        return

    if args.tag and not args.commit:
        print(
            "warning: --tag without --commit tags HEAD; make sure the synced changes are committed.",
            file=sys.stderr,
        )

    print(f"Pegg version -> {version}")
    sync.apply()
    if args.commit:
        sync.commit()
    if args.tag:
        sync.tag()
    if args.push:
        sync.push()
    print(f"Done. Review the diff, then build and tag v{version}.")


if __name__ == "__main__":
    main()
