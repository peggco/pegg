package commentline

import (
	"path/filepath"
	"strings"
)

type commentStyle struct {
	prefixes  []string
	cBlock    bool
	htmlBlock bool
}

var (
	cStyle    = commentStyle{prefixes: []string{"//"}, cBlock: true}
	hashStyle = commentStyle{prefixes: []string{"#"}}
	dashStyle = commentStyle{prefixes: []string{"--"}}
	semiStyle = commentStyle{prefixes: []string{";"}}
	pctStyle  = commentStyle{prefixes: []string{"%"}}
	cssStyle  = commentStyle{prefixes: []string{"//"}, cBlock: true}
	htmlStyle = commentStyle{htmlBlock: true}
	generic   = commentStyle{prefixes: []string{"//", "#"}, cBlock: true}
)

func styleForPath(path string) commentStyle {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext == "" {
		switch base {
		case "makefile", "dockerfile", "gemfile", "rakefile", "procfile", "cmakelists.txt", "vagrantfile":
			return hashStyle
		}
		if strings.HasPrefix(base, ".") {
			ext = strings.TrimPrefix(base, ".")
		}
	}

	switch ext {
	case "go", "js", "jsx", "mjs", "cjs", "ts", "tsx", "java", "c", "h", "cc",
		"cpp", "cxx", "hpp", "hh", "hxx", "cs", "kt", "kts", "swift", "rs",
		"scala", "sc", "dart", "groovy", "gradle", "m", "mm", "sol", "zig", "v",
		"sv", "svh", "proto", "thrift", "d", "vala":
		return cStyle

	case "css", "scss", "sass", "less":
		return cssStyle

	case "py", "pyi", "pyw", "rb", "rake", "gemspec", "sh", "bash", "zsh",
		"fish", "ksh", "pl", "pm", "pod", "yaml", "yml", "toml", "r", "ex",
		"exs", "nim", "cr", "jl", "conf", "mk", "tf", "tfvars", "hcl", "ps1",
		"psm1", "awk", "tcl", "coffee", "nix", "cmake", "gd", "graphql", "gql",
		"properties", "env", "gitignore", "editorconfig", "dockerignore", "lock":
		return hashStyle

	case "sql", "psql", "plsql", "lua", "hs", "lhs", "elm", "ada", "adb", "ads",
		"vhd", "vhdl":
		return dashStyle

	case "asm", "s", "nasm", "ini", "cfg", "lisp", "clj", "cljs", "cljc", "edn",
		"scm", "el":
		return semiStyle

	case "tex", "latex", "sty", "cls", "bib", "erl", "hrl", "pro":
		return pctStyle

	case "html", "htm", "xhtml", "xml", "svg", "xaml", "plist", "md", "markdown",
		"mdx", "astro":
		return htmlStyle

	case "vue", "svelte":
		return commentStyle{prefixes: []string{"//"}, cBlock: true, htmlBlock: true}
	case "php", "phtml":
		return commentStyle{prefixes: []string{"//", "#"}, cBlock: true, htmlBlock: true}
	}
	return generic
}

func countCommentLines(text string, st commentStyle) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	n := 0
	inC := false
	inHTML := false
	for _, raw := range strings.Split(text, "\n") {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		if inHTML {
			n++
			if strings.Contains(t, "-->") {
				inHTML = false
			}
			continue
		}
		if inC {
			n++
			if strings.Contains(t, "*/") {
				inC = false
			}
			continue
		}
		if st.htmlBlock && strings.HasPrefix(t, "<!--") {
			n++
			if !strings.Contains(t, "-->") {
				inHTML = true
			}
			continue
		}
		if st.cBlock && strings.HasPrefix(t, "/*") {
			n++
			if !strings.Contains(t, "*/") {
				inC = true
			}
			continue
		}
		for _, p := range st.prefixes {
			if strings.HasPrefix(t, p) {
				n++
				break
			}
		}
	}
	return n
}
