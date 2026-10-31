package tui

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
)

const clipboardTimeout = 2 * time.Second

func resolvePastedPaths(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var out []string
	seen := make(map[string]bool)
	add := func(candidate string) {
		path := normalizePastedPath(candidate)
		if path == "" || seen[path] {
			return
		}
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return
		}
		seen[path] = true
		out = append(out, path)
	}

	if strings.ContainsAny(text, "\r\n") {
		for _, line := range strings.Split(text, "\n") {
			add(line)
		}
	} else {
		add(text)
	}
	if len(out) > 0 {
		return out
	}

	for _, token := range splitPastedPaths(text) {
		add(token)
	}
	return out
}

func splitPastedPaths(text string) []string {
	var (
		out     []string
		current strings.Builder
		quote   rune
		escaped bool
	)
	flush := func() {
		if current.Len() > 0 {
			out = append(out, current.String())
			current.Reset()
		}
	}
	for _, r := range text {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '\n' || r == '\r' || unicode.IsSpace(r):
			flush()
		default:
			current.WriteRune(r)
		}
	}
	if escaped {
		current.WriteRune('\\')
	}
	flush()
	return out
}

func normalizePastedPath(raw string) string {
	path := strings.TrimSpace(raw)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "file://") {
		if u, err := url.Parse(path); err == nil && u.Path != "" {
			path = u.Path
			if u.Host != "" && u.Host != "localhost" {
				path = "//" + u.Host + u.Path
			}
			if decoded, err := url.PathUnescape(path); err == nil {
				path = decoded
			}
		}
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[1:])
		}
	}
	return path
}

func readClipboardImage() ([]byte, bool) {
	switch runtime.GOOS {
	case "darwin":
		return readDarwinClipboardImage()
	case "windows":
		return readWindowsClipboardImage()
	default:
		if data, ok := readWaylandClipboardImage(); ok {
			return data, true
		}
		return readX11ClipboardImage()
	}
}

func readWaylandClipboardImage() ([]byte, bool) {
	if _, err := exec.LookPath("wl-paste"); err != nil {
		return nil, false
	}
	types, ok := runClipboardCommand("wl-paste", "--list-types")
	if !ok {
		return nil, false
	}
	mime := pickImageMIME(string(types))
	if mime == "" {
		return nil, false
	}
	return runClipboardCommand("wl-paste", "--no-newline", "--type", mime)
}

func readX11ClipboardImage() ([]byte, bool) {
	if _, err := exec.LookPath("xclip"); err != nil {
		return nil, false
	}
	types, ok := runClipboardCommand("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o")
	if !ok {
		return nil, false
	}
	mime := pickImageMIME(string(types))
	if mime == "" {
		return nil, false
	}
	return runClipboardCommand("xclip", "-selection", "clipboard", "-t", mime, "-o")
}

func readDarwinClipboardImage() ([]byte, bool) {
	if _, err := exec.LookPath("osascript"); err != nil {
		return nil, false
	}
	file, err := os.CreateTemp("", "pegg-clipboard-*.png")
	if err != nil {
		return nil, false
	}
	path := file.Name()
	_ = file.Close()
	_ = os.Remove(path)
	defer os.Remove(path)

	script := `set outFile to POSIX file "` + path + `"
try
	set imgData to the clipboard as «class PNGf»
on error
	return "empty"
end try
set fh to open for access outFile with write permission
set eof of fh to 0
write imgData to fh
close access fh
return "ok"`
	if out, ok := runClipboardCommand("osascript", "-e", script); !ok || !strings.Contains(string(out), "ok") {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	return data, true
}

func readWindowsClipboardImage() ([]byte, bool) {
	script := `Add-Type -AssemblyName System.Windows.Forms
$img = [System.Windows.Forms.Clipboard]::GetImage()
if ($null -eq $img) { exit 1 }
$ms = New-Object System.IO.MemoryStream
$img.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
[Console]::OpenStandardOutput().Write($ms.ToArray(), 0, $ms.Length)`
	return runClipboardCommand("powershell", "-NoProfile", "-STA", "-Command", script)
}

func runClipboardCommand(name string, args ...string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), clipboardTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil || len(out) == 0 {
		return nil, false
	}
	return out, true
}

func pickImageMIME(types string) string {
	var fallback string
	for _, field := range strings.Fields(types) {
		mimeType := strings.ToLower(field)
		switch {
		case mimeType == "image/png":
			return mimeType
		case mimeType == "image/jpeg":
			if fallback == "" {
				fallback = mimeType
			}
		case strings.HasPrefix(mimeType, "image/") && !strings.Contains(mimeType, "svg"):
			if fallback == "" {
				fallback = mimeType
			}
		}
	}
	return fallback
}

func clipboardImageName(data []byte) string {
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return "clipboard.jpg"
	case "image/gif":
		return "clipboard.gif"
	case "image/webp":
		return "clipboard.webp"
	default:
		return "clipboard.png"
	}
}
