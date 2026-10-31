//go:build gui

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	gruntime "runtime"
	"sort"
	"strings"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/llm/subscription"
	"github.com/peggco/pegg/internal/skill"
	"github.com/peggco/pegg/internal/stt"
	"github.com/peggco/pegg/internal/tui/i18n"
	"github.com/peggco/pegg/pkg/desktop/internal/acpclient"
)

type LanguageOption struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

type ModelOption struct {
	ID               string   `json:"id"`
	Name             string   `json:"name,omitempty"`
	ReasoningEfforts []string `json:"reasoningEfforts,omitempty"`
}

type ProviderOption struct {
	Name   string        `json:"name"`
	Driver string        `json:"driver,omitempty"`
	Models []ModelOption `json:"models"`
}

type ProviderCatalogEntry struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	Kind       string `json:"kind"`
	Hint       string `json:"hint"`
}

type STTProviderEntry struct {
	Name       string   `json:"name"`
	Configured bool     `json:"configured"`
	Models     []string `json:"models"`
}

type STTSettings struct {
	Providers        []string            `json:"providers"`
	Configured       []string            `json:"configured"`
	Provider         string              `json:"provider"`
	Model            string              `json:"model"`
	Models           []string            `json:"models"`
	ModelsByProvider map[string][]string `json:"modelsByProvider"`
	Entries          []STTProviderEntry  `json:"entries"`
	RecordCommand    string              `json:"recordCommand"`
	MaxDuration      int                 `json:"maxDuration"`
}

type GeneralSettings struct {
	Provider        string           `json:"provider"`
	Model           string           `json:"model"`
	ReasoningEffort string           `json:"reasoningEffort"`
	Language        string           `json:"language"`
	Languages       []LanguageOption `json:"languages"`
	Providers       []ProviderOption `json:"providers"`
	STT             STTSettings      `json:"stt"`
}

type SessionSettings struct {
	ACPID    string `json:"acpId"`
	PeggID   string `json:"peggId"`
	Title    string `json:"title"`
	Cwd      string `json:"cwd"`
	Mode     string `json:"mode"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type CompactionSettings struct {
	Enabled            bool     `json:"enabled"`
	Strategies         []string `json:"strategies"`
	AllStrategies      []string `json:"allStrategies"`
	Threshold          float64  `json:"threshold"`
	MaxMessages        int      `json:"maxMessages"`
	MaxToolOutputChars int      `json:"maxToolOutputChars"`
}

type ToolPermission struct {
	Name  string `json:"name"`
	Mode  string `json:"mode"`
	Group string `json:"group"`
}

type PermissionsSettings struct {
	Default        string            `json:"default"`
	Modes          []string          `json:"modes"`
	Tools          []ToolPermission  `json:"tools"`
	JudgeProviders []string          `json:"judgeProviders"`
	JudgeProvider  string            `json:"judgeProvider"`
	JudgeModel     string            `json:"judgeModel"`
	JudgeThreshold float64           `json:"judgeThreshold"`
	Defaults       map[string]string `json:"defaults"`
}

type MemorySettings struct {
	Enabled       bool     `json:"enabled"`
	Provider      string   `json:"provider"`
	Model         string   `json:"model"`
	Gate          string   `json:"gate"`
	Gates         []string `json:"gates"`
	Threshold     float64  `json:"threshold"`
	ContextBudget int      `json:"contextBudget"`
	MaxResults    int      `json:"maxResults"`
	Consolidate   bool     `json:"consolidate"`
	Entries       int      `json:"entries"`
	LastEntry     string   `json:"lastEntry"`
}

type MCPServerSetting struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	URL     string   `json:"url,omitempty"`
	Env     int      `json:"env"`
	Headers int      `json:"headers"`
}

type SkillSetting struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Path        string `json:"path"`
}

type RuleSetting struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Path    string `json:"path"`
	Preview string `json:"preview"`
}

type PluginSetting struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}

type SystemSettings struct {
	AppVersion    string `json:"appVersion"`
	AgentVersion  string `json:"agentVersion"`
	Platform      string `json:"platform"`
	Arch          string `json:"arch"`
	BinaryPath    string `json:"binaryPath"`
	BinaryVersion string `json:"binaryVersion"`
	BinarySource  string `json:"binarySource"`
	ConfigPath    string `json:"configPath"`
	ConfigDir     string `json:"configDir"`
	ProjectDir    string `json:"projectDir"`
	LogPath       string `json:"logPath"`
	MemoryDir     string `json:"memoryDir"`
	SkillsDir     string `json:"skillsDir"`
	RulesDir      string `json:"rulesDir"`
	PluginsDir    string `json:"pluginsDir"`
}

type SettingsSnapshot struct {
	General        GeneralSettings     `json:"general"`
	Session        SessionSettings     `json:"session"`
	Compaction     CompactionSettings  `json:"compaction"`
	Permissions    PermissionsSettings `json:"permissions"`
	Memory         MemorySettings      `json:"memory"`
	System         SystemSettings      `json:"system"`
	PluginsEnabled bool                `json:"pluginsEnabled"`
	MCP            []MCPServerSetting  `json:"mcp"`
	Skills         []SkillSetting      `json:"skills"`
	Rules          []RuleSetting       `json:"rules"`
	Plugins        []PluginSetting     `json:"plugins"`
}

var permissionToolGroups = []struct {
	Name  string
	Group string
}{
	{"read", "Files"}, {"list", "Files"}, {"glob", "Files"}, {"grep", "Files"},
	{"write", "Files"}, {"edit", "Files"}, {"delete", "Files"},
	{"bash", "Execution"},
	{"webfetch", "Web"}, {"websearch", "Web"},
	{"task", "Agent"}, {"taskstatus", "Agent"}, {"todowrite", "Agent"}, {"todoread", "Agent"},
	{"askuserquestion", "Agent"}, {"enterplanmode", "Agent"}, {"exitplanmode", "Agent"}, {"schedule", "Agent"},
}

var compactionStrategies = []string{"tool-clearing", "sliding-window", "summarization"}

var errProviderName = errors.New("provider name is required")

func (a *App) SettingsSnapshot() (SettingsSnapshot, error) {
	cfg, err := config.Load()
	if err != nil {
		return SettingsSnapshot{}, err
	}
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	ws := a.currentWorkspace()
	globalDir, _ := config.GetConfigDir()

	snap := SettingsSnapshot{}
	snap.General = buildGeneralSettings(cfg, a)
	snap.Session = buildSessionSettings(a)
	snap.Compaction = buildCompactionSettings(cfg)
	snap.Permissions = buildPermissionSettings(cfg)
	snap.Memory = buildMemorySettings(cfg, globalDir)
	snap.MCP = buildMCPSettings(cfg)
	snap.Skills = buildSkillSettings(ws, globalDir)
	snap.Rules = buildRuleSettings(ws, globalDir)
	snap.Plugins = buildPluginSettings(cfg, globalDir)
	snap.PluginsEnabled = cfg.Plugins.Enabled
	snap.System = buildSystemSettings(a, cfg, ws, globalDir)
	normalizeSnapshot(&snap)
	return snap, nil
}

func normalizeSnapshot(snap *SettingsSnapshot) {
	if snap.General.Providers == nil {
		snap.General.Providers = []ProviderOption{}
	}
	for i := range snap.General.Providers {
		if snap.General.Providers[i].Models == nil {
			snap.General.Providers[i].Models = []ModelOption{}
		}
	}
	if snap.General.Languages == nil {
		snap.General.Languages = []LanguageOption{}
	}
	if snap.General.STT.Providers == nil {
		snap.General.STT.Providers = []string{}
	}
	if snap.General.STT.Configured == nil {
		snap.General.STT.Configured = []string{}
	}
	if snap.General.STT.Models == nil {
		snap.General.STT.Models = []string{}
	}
	if snap.General.STT.ModelsByProvider == nil {
		snap.General.STT.ModelsByProvider = map[string][]string{}
	}
	if snap.General.STT.Entries == nil {
		snap.General.STT.Entries = []STTProviderEntry{}
	}
	for i := range snap.General.STT.Entries {
		if snap.General.STT.Entries[i].Models == nil {
			snap.General.STT.Entries[i].Models = []string{}
		}
	}
	if snap.Compaction.Strategies == nil {
		snap.Compaction.Strategies = []string{}
	}
	if snap.Compaction.AllStrategies == nil {
		snap.Compaction.AllStrategies = []string{}
	}
	if snap.Permissions.Modes == nil {
		snap.Permissions.Modes = []string{}
	}
	if snap.Permissions.Tools == nil {
		snap.Permissions.Tools = []ToolPermission{}
	}
	if snap.Permissions.JudgeProviders == nil {
		snap.Permissions.JudgeProviders = []string{}
	}
	if snap.Permissions.Defaults == nil {
		snap.Permissions.Defaults = map[string]string{}
	}
	if snap.Memory.Gates == nil {
		snap.Memory.Gates = []string{}
	}
	if snap.MCP == nil {
		snap.MCP = []MCPServerSetting{}
	}
	if snap.Skills == nil {
		snap.Skills = []SkillSetting{}
	}
	if snap.Rules == nil {
		snap.Rules = []RuleSetting{}
	}
	if snap.Plugins == nil {
		snap.Plugins = []PluginSetting{}
	}
}

func buildGeneralSettings(cfg *config.Config, a *App) GeneralSettings {
	g := GeneralSettings{Language: cfg.Language}
	if g.Language == "" {
		g.Language = i18n.English
	}
	for _, code := range i18n.Names() {
		g.Languages = append(g.Languages, LanguageOption{Code: code, Label: i18n.DisplayName(code)})
	}
	for _, provider := range cfg.Providers {
		option := ProviderOption{Name: provider.Provider, Driver: provider.Driver}
		if option.Name == "" {
			option.Name = provider.Driver
		}
		for _, model := range provider.Models {
			item := ModelOption{ID: model.ID, Name: model.Name}
			if model.Config != nil {
				item.ReasoningEfforts = model.Config.ReasoningEfforts()
			}
			option.Models = append(option.Models, item)
		}
		g.Providers = append(g.Providers, option)
	}
	if session := a.CurrentSession(); session != nil {
		g.Provider = session.Provider
		g.Model = session.Model
		g.ReasoningEffort = session.ReasoningEffort
	}
	if g.Provider == "" && len(g.Providers) > 0 {
		g.Provider = g.Providers[0].Name
		if len(g.Providers[0].Models) > 0 {
			g.Model = g.Providers[0].Models[0].ID
		}
	}
	g.STT = buildSTTSettings(cfg.STT)
	return g
}

func buildSTTSettings(cfg *config.STTConfig) STTSettings {
	out := STTSettings{MaxDuration: config.DefaultSTTMaxDurationSecs}
	if cfg != nil {
		out.Provider = cfg.Provider
		out.Model = cfg.Model
		out.RecordCommand = cfg.RecordCommand
		out.MaxDuration = cfg.MaxDurationValue()
		for _, provider := range cfg.Providers {
			if provider.Provider != "" {
				out.Configured = append(out.Configured, provider.Provider)
			}
		}
		sort.Strings(out.Configured)
	}
	out.Providers = stt.ProvidersForConfig(cfg)
	out.ModelsByProvider = map[string][]string{}
	configured := map[string]bool{}
	for _, name := range out.Configured {
		configured[name] = true
	}
	out.Entries = make([]STTProviderEntry, 0, len(out.Providers))
	for _, provider := range out.Providers {
		var models []string
		for _, model := range stt.ModelsForConfig(provider, cfg) {
			models = append(models, model.ID)
		}
		out.ModelsByProvider[provider] = models
		if provider == out.Provider {
			out.Models = models
		}
		out.Entries = append(out.Entries, STTProviderEntry{
			Name:       provider,
			Configured: configured[provider],
			Models:     models,
		})
	}
	return out
}

func buildSessionSettings(a *App) SessionSettings {
	session := a.CurrentSession()
	if session == nil {
		return SessionSettings{}
	}
	return SessionSettings{
		ACPID:    session.ACPID,
		PeggID:   session.PeggID,
		Title:    session.Title,
		Cwd:      session.Cwd,
		Mode:     session.Mode,
		Provider: session.Provider,
		Model:    session.Model,
	}
}

func buildCompactionSettings(cfg *config.Config) CompactionSettings {
	out := CompactionSettings{
		Enabled:            true,
		AllStrategies:      append([]string(nil), compactionStrategies...),
		Strategies:         []string{"tool-clearing", "sliding-window"},
		Threshold:          80,
		MaxMessages:        50,
		MaxToolOutputChars: 4000,
	}
	if cfg.Compaction != nil {
		out.Enabled = cfg.Compaction.Enabled
		out.Threshold = cfg.Compaction.Threshold
		out.MaxMessages = cfg.Compaction.MaxMessages
		out.MaxToolOutputChars = cfg.Compaction.MaxToolOutputChars
		if len(cfg.Compaction.Strategy) > 0 {
			out.Strategies = append([]string(nil), cfg.Compaction.Strategy...)
		}
	}
	return out
}

func buildPermissionSettings(cfg *config.Config) PermissionsSettings {
	out := PermissionsSettings{
		Default:        "semi-ask",
		Modes:          []string{"allow", "semi-ask", "ask", "semi-judge", "judge"},
		JudgeThreshold: config.DefaultJudgeThreshold,
	}
	defaults := config.DefaultConfig().Permission
	rules := map[string]string{}
	if defaults != nil {
		out.Default = defaults.Default
		for name, mode := range defaults.Rules {
			rules[name] = mode
		}
	}
	out.Defaults = map[string]string{}
	if defaults != nil {
		for name, mode := range defaults.Rules {
			out.Defaults[name] = mode
		}
	}
	if cfg.Permission != nil {
		if cfg.Permission.Default != "" {
			out.Default = cfg.Permission.Default
		}
		for name, mode := range cfg.Permission.Rules {
			rules[name] = mode
		}
		out.JudgeProvider = cfg.Permission.JudgeProvider
		out.JudgeModel = cfg.Permission.JudgeModel
		out.JudgeThreshold = cfg.Permission.JudgeThresholdValue()
	}
	for _, tool := range permissionToolGroups {
		mode := rules[tool.Name]
		if mode == "" {
			mode = out.Default
		}
		out.Tools = append(out.Tools, ToolPermission{Name: tool.Name, Mode: mode, Group: tool.Group})
	}
	seen := map[string]bool{}
	for _, provider := range cfg.Providers {
		name := provider.Provider
		if name == "" {
			name = provider.Driver
		}
		if name != "" && !seen[name] {
			seen[name] = true
			out.JudgeProviders = append(out.JudgeProviders, name)
		}
	}
	sort.Strings(out.JudgeProviders)
	return out
}

func buildMemorySettings(cfg *config.Config, globalDir string) MemorySettings {
	out := MemorySettings{
		Enabled:       true,
		Gate:          config.GateAuto,
		Gates:         config.GateModes(),
		Threshold:     config.DefaultMemoryThreshold,
		ContextBudget: config.DefaultMemoryBudget,
		MaxResults:    config.DefaultMemoryMaxResults,
		Consolidate:   true,
	}
	if cfg.Memory != nil {
		out.Enabled = cfg.Memory.Enabled
		out.Provider = cfg.Memory.Provider
		out.Model = cfg.Memory.Model
		out.Gate = cfg.Memory.GateValue()
		out.Threshold = cfg.Memory.ThresholdValue()
		out.ContextBudget = cfg.Memory.BudgetValue()
		out.MaxResults = cfg.Memory.MaxResultsValue()
		out.Consolidate = cfg.Memory.ConsolidateValue()
	}
	out.Entries, out.LastEntry = memoryStats(filepath.Join(globalDir, "memory"))
	return out
}

func memoryStats(dir string) (int, string) {
	var count int
	var newest time.Time
	var newestName string
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			return nil
		}
		count++
		if info, err := entry.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
			rel, _ := filepath.Rel(dir, path)
			newestName = rel
		}
		return nil
	})
	if !newest.IsZero() {
		return count, newest.Format("2006-01-02 15:04") + " · " + newestName
	}
	return count, ""
}

func buildMCPSettings(cfg *config.Config) []MCPServerSetting {
	names := make([]string, 0, len(cfg.MCPServers))
	for name := range cfg.MCPServers {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]MCPServerSetting, 0, len(names))
	for _, name := range names {
		server := cfg.MCPServers[name]
		kind := "stdio"
		if server.URL != "" {
			kind = "http"
		}
		out = append(out, MCPServerSetting{
			Name:    name,
			Type:    kind,
			Command: server.Command,
			Args:    server.Args,
			URL:     server.URL,
			Env:     len(server.Env),
			Headers: len(server.Headers),
		})
	}
	return out
}

func loadDesktopSkills(workspace, globalDir string) {
	globalSkills := filepath.Join(globalDir, "skills")
	_ = skill.MaterializeTo(globalSkills)
	home, _ := os.UserHomeDir()
	_ = skill.LoadDir(filepath.Join(home, ".agents", "skills"), ".agents")
	_ = skill.LoadDir(globalSkills, "global")
	_ = skill.LoadDir(filepath.Join(workspace, ".pegg", "skills"), "project")
}

func buildSkillSettings(workspace, globalDir string) []SkillSetting {
	loadDesktopSkills(workspace, globalDir)
	skills := skill.List()
	out := make([]SkillSetting, 0, len(skills))
	for _, item := range skills {
		out = append(out, SkillSetting{
			Name:        item.Name,
			Description: item.Description,
			Source:      item.Source,
			Path:        item.Path,
		})
	}
	return out
}

func buildRuleSettings(workspace, globalDir string) []RuleSetting {
	var out []RuleSetting
	collect := func(dir, source string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			preview := ""
			if raw, err := os.ReadFile(path); err == nil {
				preview = truncateText(string(raw), 900)
			}
			out = append(out, RuleSetting{Name: entry.Name(), Source: source, Path: path, Preview: preview})
		}
	}
	collect(filepath.Join(globalDir, "rules"), "global")
	collect(filepath.Join(workspace, ".pegg", "rules"), "project")
	return out
}

func buildPluginSettings(cfg *config.Config, globalDir string) []PluginSetting {
	dir := filepath.Join(globalDir, "plugins")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	excluded := map[string]bool{}
	for _, name := range cfg.Plugins.Exclude {
		excluded[name] = true
	}
	out := make([]PluginSetting, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		enabled := cfg.Plugins.Enabled && !excluded[entry.Name()]
		status := "enabled"
		if !enabled {
			status = "disabled"
		}
		out = append(out, PluginSetting{
			Name:    entry.Name(),
			Path:    filepath.Join(dir, entry.Name()),
			Enabled: enabled,
			Status:  status,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func buildSystemSettings(a *App, cfg *config.Config, workspace, globalDir string) SystemSettings {
	agentVersion := ""
	a.mu.Lock()
	if a.initRes != nil && a.initRes.AgentInfo != nil {
		agentVersion = a.initRes.AgentInfo.Version
	}
	binary := a.binary
	a.mu.Unlock()
	configPath, _ := config.GetConfigPath("pegg.json")
	return SystemSettings{
		AppVersion:    appVersion,
		AgentVersion:  agentVersion,
		Platform:      gruntime.GOOS,
		Arch:          gruntime.GOARCH,
		BinaryPath:    binary.Path,
		BinaryVersion: binary.Version,
		BinarySource:  binary.Source,
		ConfigPath:    configPath,
		ConfigDir:     globalDir,
		ProjectDir:    filepath.Join(workspace, ".pegg"),
		LogPath:       a.LogPath(),
		MemoryDir:     filepath.Join(globalDir, "memory"),
		SkillsDir:     filepath.Join(workspace, ".pegg", "skills"),
		RulesDir:      filepath.Join(workspace, ".pegg", "rules"),
		PluginsDir:    filepath.Join(globalDir, "plugins"),
	}
}

func truncateText(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}

func (a *App) AddSTTProvider(name, apiKey, baseURL string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errProviderName
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.STT == nil {
		cfg.STT = &config.STTConfig{MaxDurationSecs: config.DefaultSTTMaxDurationSecs}
	}
	known := false
	for _, provider := range stt.ProvidersForConfig(cfg.STT) {
		if provider == name {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("unknown speech-to-text provider %q", name)
	}
	entry, exists := cfg.STT.ProviderConfig(name)
	if !exists {
		entry = config.STTProviderConfig{Provider: name}
	}
	if key := strings.TrimSpace(apiKey); key != "" {
		entry.APIKey = key
	}
	if base := strings.TrimSpace(baseURL); base != "" {
		entry.BaseURL = base
	}
	if !exists && entry.APIKey == "" && !strings.HasPrefix(name, "pegg-team") {
		return fmt.Errorf("API key is required for %s", name)
	}
	cfg.STT.UpsertProvider(entry)
	cfg.STT.Provider = name
	knownModel := false
	for _, model := range stt.ModelsForConfig(name, cfg.STT) {
		if model.ID == cfg.STT.Model {
			knownModel = true
			break
		}
	}
	if !knownModel {
		cfg.STT.Model = stt.DefaultModelForConfig(name, cfg.STT)
	}
	return config.Save(cfg)
}

func (a *App) RemoveSTTProvider(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errProviderName
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.STT == nil {
		return fmt.Errorf("speech-to-text is not configured")
	}
	next := cfg.STT.Providers[:0]
	removed := false
	for _, provider := range cfg.STT.Providers {
		if provider.Provider == name {
			removed = true
			continue
		}
		next = append(next, provider)
	}
	if !removed {
		return fmt.Errorf("speech-to-text provider %q is not configured", name)
	}
	cfg.STT.Providers = next
	if cfg.STT.Provider == name {
		cfg.STT.Provider = ""
		cfg.STT.Model = ""
	}
	return config.Save(cfg)
}

func (a *App) SaveGeneralSettings(g GeneralSettings) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if g.Language != "" {
		cfg.Language = g.Language
	}
	if cfg.STT == nil {
		cfg.STT = &config.STTConfig{MaxDurationSecs: config.DefaultSTTMaxDurationSecs}
	}
	if g.STT.Provider == "" {
		cfg.STT.Provider = ""
		cfg.STT.Model = ""
	} else {
		cfg.STT.Provider = g.STT.Provider
		cfg.STT.Model = g.STT.Model
		knownModel := false
		for _, model := range stt.ModelsForConfig(cfg.STT.Provider, cfg.STT) {
			if model.ID == cfg.STT.Model {
				knownModel = true
				break
			}
		}
		if !knownModel {
			cfg.STT.Model = stt.DefaultModelForConfig(cfg.STT.Provider, cfg.STT)
		}
	}
	if g.STT.RecordCommand != "" || g.STT.Provider != "" {
		cfg.STT.RecordCommand = strings.TrimSpace(g.STT.RecordCommand)
	}
	if g.STT.MaxDuration > 0 {
		cfg.STT.MaxDurationSecs = g.STT.MaxDuration
	}
	return config.Save(cfg)
}

func (a *App) SaveCompactionSettings(c CompactionSettings) error {
	threshold := c.Threshold
	if threshold < 10 {
		threshold = 10
	}
	if threshold > 100 {
		threshold = 100
	}
	maxMessages := c.MaxMessages
	if maxMessages < 5 {
		maxMessages = 5
	}
	maxTool := c.MaxToolOutputChars
	if maxTool < 500 {
		maxTool = 500
	}
	valid := map[string]bool{}
	for _, name := range compactionStrategies {
		valid[name] = true
	}
	strategies := make([]string, 0, len(c.Strategies))
	for _, name := range c.Strategies {
		if valid[name] {
			strategies = append(strategies, name)
		}
	}
	next := &config.CompactionConfig{
		Enabled:            c.Enabled,
		Strategy:           strategies,
		Threshold:          threshold,
		MaxMessages:        maxMessages,
		MaxToolOutputChars: maxTool,
	}
	if existing, err := config.Load(); err == nil && existing.Compaction != nil {
		next.SummarizerProvider = existing.Compaction.SummarizerProvider
		next.SummarizerModel = existing.Compaction.SummarizerModel
	}
	return config.UpsertCompaction(next)
}

func (a *App) SavePermissionSettings(p PermissionsSettings) error {
	validModes := map[string]bool{"allow": true, "semi-ask": true, "ask": true, "semi-judge": true, "judge": true}
	def := p.Default
	if !validModes[def] {
		def = "semi-ask"
	}
	rules := make(map[string]string, len(p.Tools))
	for _, tool := range p.Tools {
		if tool.Name == "" {
			continue
		}
		mode := tool.Mode
		if !validModes[mode] {
			mode = def
		}
		rules[tool.Name] = mode
	}
	threshold := p.JudgeThreshold
	if threshold < 0 || threshold > 1 {
		threshold = config.DefaultJudgeThreshold
	}
	return config.UpsertPermission(&config.PermissionConfig{
		Default:        def,
		JudgeProvider:  strings.TrimSpace(p.JudgeProvider),
		JudgeModel:     strings.TrimSpace(p.JudgeModel),
		JudgeThreshold: &threshold,
		Rules:          rules,
	})
}

func (a *App) SaveMemorySettings(m MemorySettings) error {
	threshold := m.Threshold
	if threshold < 0 || threshold > 1 {
		threshold = config.DefaultMemoryThreshold
	}
	budget := m.ContextBudget
	if budget < 500 {
		budget = config.DefaultMemoryBudget
	}
	maxResults := m.MaxResults
	if maxResults < 1 {
		maxResults = config.DefaultMemoryMaxResults
	}
	consolidate := m.Consolidate
	return config.UpsertMemory(&config.MemoryConfig{
		Enabled:       m.Enabled,
		Provider:      strings.TrimSpace(m.Provider),
		Model:         strings.TrimSpace(m.Model),
		Gate:          m.Gate,
		Threshold:     &threshold,
		ContextBudget: budget,
		MaxResults:    maxResults,
		Consolidate:   &consolidate,
	})
}

func (a *App) TogglePluginSettings(name string) (bool, error) {
	return config.TogglePlugin(name)
}

func (a *App) ProviderCatalog() []ProviderCatalogEntry {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.DefaultConfig()
	}
	configured := map[string]bool{}
	for _, provider := range cfg.Providers {
		if provider.Provider != "" {
			configured[provider.Provider] = true
		}
	}
	out := make([]ProviderCatalogEntry, 0)
	for _, name := range llm.ListProviderNames(cfg.Providers) {
		kind := "api_key"
		hint := "API key"
		if info, ok := subscription.Get(name); ok {
			kind = "subscription"
			hint = info.Hint
			if info.Status().LoggedIn {
				hint = "Signed in"
			}
		}
		out = append(out, ProviderCatalogEntry{Name: name, Configured: configured[name], Kind: kind, Hint: hint})
	}
	return out
}

func (a *App) AddProvider(name, apiKey, baseURL string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errProviderName
	}
	catalog := a.ProviderCatalog()
	known := false
	var entry ProviderCatalogEntry
	for _, item := range catalog {
		if item.Name == name {
			known = true
			entry = item
			break
		}
	}
	if !known {
		return fmt.Errorf("unknown provider %q", name)
	}
	if entry.Kind != "subscription" && strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("API key is required for %s", name)
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var current config.LLMConfig
	found := false
	for _, provider := range cfg.Providers {
		if provider.Provider == name {
			current = provider
			found = true
			break
		}
	}
	if !found {
		current = config.LLMConfig{Provider: name}
	}
	if strings.TrimSpace(apiKey) != "" {
		current.APIKey = strings.TrimSpace(apiKey)
	}
	if strings.TrimSpace(baseURL) != "" {
		current.BaseURL = strings.TrimSpace(baseURL)
	}
	return config.UpsertProvider(current)
}

func (a *App) RemoveProvider(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errProviderName
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	next := cfg.Providers[:0]
	removed := false
	for _, provider := range cfg.Providers {
		if provider.Provider == name {
			removed = true
			continue
		}
		next = append(next, provider)
	}
	if !removed {
		return fmt.Errorf("provider %q is not configured", name)
	}
	cfg.Providers = next
	return config.Save(cfg)
}

func (a *App) SlashCommands() []acpclient.SlashCommand {
	globalDir, _ := config.GetConfigDir()
	loadDesktopSkills(a.currentWorkspace(), globalDir)
	skills := skill.List()
	out := make([]acpclient.SlashCommand, 0, len(skills))
	for _, sk := range skills {
		desc := sk.Description
		if desc == "" {
			desc = sk.WhenToUse
		}
		out = append(out, acpclient.SlashCommand{Name: sk.Name, Description: desc})
	}
	return out
}
