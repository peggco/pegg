package cli

import (
	"strconv"
	"strings"
)

const pluginReleaseWorkflowTemplate = `name: Release

on:
  push:
    tags:
      - "v*"
  workflow_dispatch:
    inputs:
      version:
        description: "Release tag (for example v0.1.0)"
        required: true

jobs:
  release:
    permissions:
      contents: write
    uses: __SLUG__/.github/workflows/plugin-release.yml@__VERSION__
    with:
      binary: __BINARY__
      version: ${{ github.ref_type == 'tag' && github.ref_name || inputs.version }}
    secrets:
      token: ${{ secrets.GITHUB_TOKEN }}
`

const pluginGitignoreTemplate = `# Build output
/bin/
/dist/
/build/
/out/
/release/

# Test and coverage output
*.test
*.out
coverage.out

# Local debug binaries
debug
/__BINARY__

# Environment
.env
.env.local

# Editors
.vscode/
.idea/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Logs and temp
*.log
tmp/
temp/

# Go workspace
go.work
go.work.sum
`

func pluginBinaryName(name string) string {
	slug := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r >= '0' && r <= '9':
			return r
		case r == '.', r == '_', r == '-':
			return r
		default:
			return '-'
		}
	}, strings.TrimSpace(name))
	slug = strings.Trim(slug, "-")
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	if slug == "" {
		slug = "plugin"
	}
	return "pegg-" + slug
}

func renderPluginReleaseWorkflow(binary, module, version string) string {
	return strings.NewReplacer(
		"__BINARY__", binary,
		"__SLUG__", githubRepoSlug(module),
		"__VERSION__", version,
	).Replace(pluginReleaseWorkflowTemplate)
}

func githubRepoSlug(module string) string {
	slug := strings.TrimPrefix(strings.TrimSpace(module), "https://")
	slug = strings.TrimPrefix(slug, "github.com/")
	slug = strings.TrimSuffix(slug, "/")
	if slug == "" {
		return "peggco/pegg"
	}
	return slug
}

func renderPluginGitignore(binary string) string {
	return strings.ReplaceAll(pluginGitignoreTemplate, "__BINARY__", binary)
}

const pluginGoTemplate = `package main

import (
	"fmt"

	peggplugin "__PEGG__/pkg/plugin"
)

type Plugin struct {
	peggplugin.Base
}

func (p *Plugin) Name() string        { return __PLUGIN_NAME__ }
func (p *Plugin) Version() string     { return "0.1.0" }
func (p *Plugin) Description() string { return "A Pegg plugin" }

func (p *Plugin) Capabilities() []string {
	return []string{
		string(peggplugin.CapabilityTools),
		string(peggplugin.CapabilitySkills),
	}
}

func (p *Plugin) Tools() ([]peggplugin.ToolSpec, error) {
	schema := "{\"type\":\"object\",\"properties\":{\"text\":{\"type\":\"string\"}},\"required\":[\"text\"]}"
	return []peggplugin.ToolSpec{{
		Name:        __TOOL_NAME__,
		Description: "Example tool. Replace this with your plugin's tool.",
		Parameters:  schema,
		Bundle:      "example",
		Group:       "Example",
		ReadOnly:    true,
	}}, nil
}

func (p *Plugin) Skills() ([]peggplugin.SkillSpec, error) {
	return []peggplugin.SkillSpec{pluginSkill()}, nil
}

func (p *Plugin) ExecuteTool(req peggplugin.ToolRequest) (*peggplugin.ToolResult, error) {
	if req.Name != __TOOL_NAME__ {
		return nil, fmt.Errorf("unknown tool %q", req.Name)
	}
	return &peggplugin.ToolResult{Output: "hello from __NAME__"}, nil
}
`

const pluginSkillTemplate = `package main

import peggplugin "__PEGG__/pkg/plugin"

func pluginSkill() peggplugin.SkillSpec {
	return peggplugin.SkillSpec{
		Name:         __PLUGIN_NAME__,
		Description:  "Use __NAME__ through the __NAME___* tools.",
		WhenToUse:    "Load this skill before calling any __NAME___* tool.",
		ToolBundles:  []string{"example"},
		Context:      "inline",
		AllowedTools: []string{__TOOL_NAME__},
		Instructions: "# __NAME__\n\nDescribe when and how to use the __NAME___* tools here.\n\n1. Call the __NAME___echo tool to verify the plugin is configured.\n2. Replace this example with the real workflow for your plugin.\n",
	}
}
`

const pluginCommandsTemplate = `package main

func runCLI(args []string) int {
	println("This plugin has no CLI commands yet.")
	return 0
}
`

func renderPluginGo(pegg, name string) string {
	return strings.NewReplacer(
		"__PEGG__", pegg,
		"__PLUGIN_NAME__", strconv.Quote(name),
		"__TOOL_NAME__", strconv.Quote(name+"_echo"),
		"__NAME__", name,
	).Replace(pluginGoTemplate)
}

func renderPluginSkill(pegg, name string) string {
	return strings.NewReplacer(
		"__PEGG__", pegg,
		"__PLUGIN_NAME__", strconv.Quote(name),
		"__TOOL_NAME__", strconv.Quote(name+"_echo"),
		"__NAME__", name,
	).Replace(pluginSkillTemplate)
}
