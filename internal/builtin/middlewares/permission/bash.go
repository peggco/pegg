package permission

import (
	"strings"

	json "github.com/goccy/go-json"
)

var commandWhitelist = map[string]bool{
	"ls": true, "pwd": true, "cat": true, "grep": true,
	"find": true, "head": true, "tail": true, "tree": true,
	"echo": true, "type": true,
	"dir": true, "findstr": true,
}

var commandBlacklist = map[string]bool{
	"rm": true, "mv": true, "cp": true, "chmod": true, "chown": true,
	"sudo": true, "su": true, "bash": true, "sh": true, "zsh": true,
	"wget": true, "curl": true, "nc": true, "kill": true, "pkill": true,
	"reboot": true, "halt": true, "poweroff": true, "dd": true, "mkfs": true,
	"docker": true, "kubectl": true,
	"del": true, "erase": true, "rmdir": true, "rd": true,
	"move": true, "ren": true, "rename": true,
	"cmd": true, "powershell": true, "pwsh": true,
	"reg": true, "regedit": true,
	"taskkill": true, "schtasks": true,
	"wmic": true, "certutil": true, "vssadmin": true,
	"format": true, "diskpart": true, "net": true, "netstat": true,
}

var dangerousArgs = []string{
	"-rf", "--force",
	"../", "..\\",
	"/etc", "/var", "/root", "/bin", "/sbin", "/usr",
	".ssh", ".gnupg",
	"c:\\windows", "\\system32", "\\syswow64",
	"hklm\\", "hkcu\\",
}

var unsafeArgs = []string{
	"-exec", "-execdir", "-ok", "-okdir", "-delete",
	"-fprint", "-fprint0", "-fprintf", "-fls",
	"--output",
}

func bashAllowed(args string) bool {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return false
	}

	cmd := strings.TrimSpace(params.Command)
	if cmd == "" {
		return false
	}

	if hasShellMetachar(cmd) {
		return false
	}

	tokens, hasGlob, ok := splitShellWords(cmd)
	if !ok || len(tokens) == 0 {
		return false
	}

	binPath := tokens[0]
	if strings.ContainsAny(binPath, "/\\") {
		return false
	}
	baseCmd := strings.ToLower(binPath)

	baseCmd = strings.TrimSuffix(baseCmd, ".exe")
	baseCmd = strings.TrimSuffix(baseCmd, ".bat")
	baseCmd = strings.TrimSuffix(baseCmd, ".cmd")
	baseCmd = strings.TrimSuffix(baseCmd, ".ps1")
	baseCmd = strings.TrimSuffix(baseCmd, ".vbs")

	if commandBlacklist[baseCmd] {
		return false
	}

	if !commandWhitelist[baseCmd] {
		return false
	}

	if (baseCmd == "find" || baseCmd == "tree") && hasGlob {
		return false
	}

	if hasDangerousArgs(tokens[1:]) {
		return false
	}

	for _, arg := range tokens[1:] {
		if isUnsafeArg(arg) {
			return false
		}
		if baseCmd == "tree" && strings.EqualFold(arg, "-o") {
			return false
		}
	}

	return true
}

func hasShellMetachar(s string) bool {
	return strings.ContainsAny(s, ";|&><$`\n\r(){}\\")
}

func splitShellWords(s string) (tokens []string, hasGlob bool, ok bool) {
	var word strings.Builder
	inWord := false
	var quote byte

	flush := func() {
		if inWord {
			tokens = append(tokens, word.String())
			word.Reset()
			inWord = false
		}
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch quote {
		case '\'':
			if c == '\'' {
				quote = 0
			} else {
				word.WriteByte(c)
			}
		case '"':
			if c == '"' {
				quote = 0
				continue
			}
			if c == '\\' && i+1 < len(s) {
				switch next := s[i+1]; next {
				case '"', '\\', '$', '`':
					word.WriteByte(next)
					i++
					continue
				}
			}
			word.WriteByte(c)
		default:
			switch c {
			case '\'', '"':
				quote = c
				inWord = true
			case '\\':
				if i+1 < len(s) {
					word.WriteByte(s[i+1])
					i++
					inWord = true
				}
			case ' ', '\t':
				flush()
			case '*', '?', '[':
				hasGlob = true
				word.WriteByte(c)
				inWord = true
			default:
				word.WriteByte(c)
				inWord = true
			}
		}
	}

	if quote != 0 {
		return nil, false, false
	}
	flush()
	return tokens, hasGlob, true
}

func isUnsafeArg(arg string) bool {
	name := strings.ToLower(arg)
	if i := strings.IndexByte(name, '='); i >= 0 {
		name = name[:i]
	}
	for _, unsafe := range unsafeArgs {
		if name == unsafe {
			return true
		}
	}
	return false
}

func hasDangerousArgs(args []string) bool {
	for _, arg := range args {
		lowerArg := strings.ToLower(arg)

		if lowerArg == "-rf" || lowerArg == "-f" || lowerArg == "--force" {
			return true
		}

		for _, danger := range dangerousArgs {
			if strings.Contains(lowerArg, danger) {
				return true
			}
		}
	}
	return false
}
