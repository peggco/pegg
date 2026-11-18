package permission

import (
	"testing"
)

func TestBashAllowed(t *testing.T) {
	cases := []struct {
		name string
		args string
		want bool
	}{
		{"ls", `{"command": "ls -la"}`, true},
		{"pwd", `{"command": "pwd"}`, true},
		{"cat", `{"command": "cat README.md"}`, true},
		{"grep", `{"command": "grep -rn TODO internal"}`, true},
		{"find", `{"command": "find . -name '*.go' -type f"}`, true},
		{"head", `{"command": "head -n 20 README.md"}`, true},
		{"tail", `{"command": "tail -n 20 README.md"}`, true},
		{"tree", `{"command": "tree -L 2"}`, true},
		{"tree output", `{"command": "tree -o /tmp/out"}`, false},
		{"tree output long", `{"command": "tree --output=/tmp/out"}`, false},
		{"echo", `{"command": "echo hello"}`, true},
		{"go test", `{"command": "go test ./..."}`, false},
		{"go run", `{"command": "go run ./cmd/pegg"}`, false},
		{"npm run build", `{"command": "npm run build"}`, false},
		{"npm install", `{"command": "npm install"}`, false},
		{"npx", `{"command": "npx cowsay hello"}`, false},
		{"node script", `{"command": "node index.js"}`, false},
		{"node eval", `{"command": "node -e 'require(\"child_process\").execSync(\"id\")'"}`, false},
		{"yarn", `{"command": "yarn build"}`, false},
		{"pnpm", `{"command": "pnpm build"}`, false},
		{"git status", `{"command": "git status"}`, false},
		{"git log", `{"command": "git log --oneline"}`, false},
		{"git pager override", `{"command": "git -c core.pager='sh -c id' log"}`, false},
		{"find exec", `{"command": "find . -name x -exec id +"}`, false},
		{"find execdir", `{"command": "find . -name x -execdir id +"}`, false},
		{"find ok", `{"command": "find . -name x -ok id +"}`, false},
		{"find delete", `{"command": "find . -name x -delete"}`, false},
		{"find fprint", `{"command": "find . -name x -fprint /tmp/out"}`, false},
		{"python", `{"command": "python -c 'print(1)'"}`, false},
		{"shell", `{"command": "sh -c id"}`, false},
		{"mkdir", `{"command": "mkdir build"}`, false},
		{"touch", `{"command": "touch file"}`, false},
		{"cat outside", `{"command": "cat /etc/passwd"}`, false},
		{"rm", `{"command": "rm -rf ."}`, false},
		{"rm with whitespace", `{"command": "  rm -rf .  "}`, false},
		{"semicolon", `{"command": "ls; rm -rf ."}`, false},
		{"pipe", `{"command": "ls | grep x"}`, false},
		{"redirection", `{"command": "ls > /tmp/out"}`, false},
		{"dollar expansion", `{"command": "ls $HOME"}`, false},
		{"command substitution", "{\"command\": \"ls `pwd`\"}", false},
		{"andand", `{"command": "ls && rm -rf ."}`, false},
		{"newline", "{\"command\": \"ls\\nrm -rf .\"}", false},
		{"absolute path", `{"command": "/bin/ls"}`, false},
		{"leading dir", `{"command": "./ls"}`, false},
		{"empty", `{"command": ""}`, false},
		{"invalid json", `not json`, false},
	}
	for _, tc := range cases {
		if got := bashAllowed(tc.args); got != tc.want {
			t.Errorf("%s: bashAllowed = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIsUnsafeArg(t *testing.T) {
	cases := []struct {
		arg  string
		want bool
	}{
		{"-exec", true},
		{"-execdir", true},
		{"-ok", true},
		{"-delete", true},
		{"-fprint0", true},
		{"-c", true},
		{"--pager", true},
		{"--output=/tmp/out", true},
		{"--exec-path=/tmp/bin", true},
		{"-name", false},
		{"-color", false},
		{"--format=json", false},
	}
	for _, tc := range cases {
		if got := isUnsafeArg(tc.arg); got != tc.want {
			t.Errorf("isUnsafeArg(%q) = %v, want %v", tc.arg, got, tc.want)
		}
	}
}

func TestHasShellMetachar(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"ls -la", false},
		{"a;b", true},
		{"a|b", true},
		{"a&b", true},
		{"a>b", true},
		{"a<b", true},
		{"a$b", true},
		{"a`b", true},
		{"a\nb", true},
		{"a\rb", true},
		{"a&&b", true},
		{"a||b", true},
	}
	for _, tc := range cases {
		if got := hasShellMetachar(tc.in); got != tc.want {
			t.Errorf("hasShellMetachar(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
