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
		{"find exec", `{"command": "find . -name x -exec id +"}`, false},
		{"find quoted exec", `{"command": "find . -name x \"-exec\" id +"}`, false},
		{"find split quoted exec", `{"command": "find . -name x -e\"xec\" id +"}`, false},
		{"find glob brace exec", `{"command": "find . \"-exec\" sh x * +"}`, false},
		{"find quoted delete", `{"command": "find . -name x '-delete'"}`, false},
		{"find quoted fprint", `{"command": "find . '-fprintf' /tmp/out '%p'"}`, false},
		{"find unquoted glob", `{"command": "find . -name *.go"}`, false},
		{"tree quoted output", `{"command": "tree \"-o\" /tmp/out"}`, false},
		{"grep count", `{"command": "grep -c TODO README.md"}`, true},
		{"grep context", `{"command": "grep -C 2 TODO README.md"}`, true},
		{"ls columns", `{"command": "ls -C"}`, true},
		{"head bytes", `{"command": "head -c 10 README.md"}`, true},
		{"cat quoted sensitive", `{"command": "cat /e\"tc\"/passwd"}`, false},
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
		{"--output=/tmp/out", true},
		{"-c", false},
		{"--pager", false},
		{"--exec-path=/tmp/bin", false},
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

func TestSplitShellWords(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		tokens  []string
		hasGlob bool
		ok      bool
	}{
		{"plain", "ls -la", []string{"ls", "-la"}, false, true},
		{"single quoted glob", "find . -name '*.go'", []string{"find", ".", "-name", "*.go"}, false, true},
		{"unquoted glob", "find . -name *.go", []string{"find", ".", "-name", "*.go"}, true, true},
		{"quoted exec", `find . "-exec" id`, []string{"find", ".", "-exec", "id"}, false, true},
		{"split quoted exec", `find . -e"xec" id`, []string{"find", ".", "-exec", "id"}, false, true},
		{"quoted path", `cat "/etc/passwd"`, []string{"cat", "/etc/passwd"}, false, true},
		{"split quoted path", `cat /e"tc"/passwd`, []string{"cat", "/etc/passwd"}, false, true},
		{"empty word", `ls ""`, []string{"ls", ""}, false, true},
		{"unterminated quote", `cat 'x`, nil, false, false},
	}
	for _, tc := range cases {
		tokens, hasGlob, ok := splitShellWords(tc.in)
		if ok != tc.ok || hasGlob != tc.hasGlob {
			t.Errorf("%s: splitShellWords(%q) ok=%v glob=%v, want ok=%v glob=%v", tc.name, tc.in, ok, hasGlob, tc.ok, tc.hasGlob)
			continue
		}
		if !ok {
			continue
		}
		if len(tokens) != len(tc.tokens) {
			t.Errorf("%s: splitShellWords(%q) = %q, want %q", tc.name, tc.in, tokens, tc.tokens)
			continue
		}
		for i := range tokens {
			if tokens[i] != tc.tokens[i] {
				t.Errorf("%s: splitShellWords(%q)[%d] = %q, want %q", tc.name, tc.in, i, tokens[i], tc.tokens[i])
			}
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
