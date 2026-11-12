package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"

	"github.com/peggco/pegg/internal/agent/tool"
	agenttools "github.com/peggco/pegg/internal/agent/tools"
	"github.com/peggco/pegg/internal/core/cache"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/lsp"
	"github.com/peggco/pegg/internal/mcp"
	"github.com/peggco/pegg/internal/plugin/bundle"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/skill"
	"github.com/peggco/pegg/internal/vfs"
)

type Manager struct {
	mu          sync.RWMutex
	log         *logger.Logger
	cfg         *config.Config
	bus         event.Bus
	fs          *vfs.VFS
	plugins     map[string]*PluginInstance
	clients     map[string]*goplugin.Client
	exclude     map[string]bool
	commands    []PluginCommand
	eventUnsubs []func() error
}

type logWriter struct {
	log *logger.Logger
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.log.Finfo("plugin: %s", strings.TrimSpace(string(p)))
	return len(p), nil
}

type PluginInstance struct {
	Name            string
	Path            string
	Version         string
	Description     string
	Client          *goplugin.Client
	Plugin          pkgplugin.Plugin
	Capabilities    []string
	Tools           []pkgplugin.ToolSpec
	Skills          []pkgplugin.SkillSpec
	PermissionRules map[string]string
	Commands        []pkgplugin.CommandSpec

	execMu  sync.Mutex
	eventMu sync.Mutex
}

type PluginCommand struct {
	Plugin string
	Spec   pkgplugin.CommandSpec
}

func NewManager(
	log *logger.Logger,
	cfg *config.Config,
	bus event.Bus,
	_ cache.Cache,
	_ *llm.Manager,
	_ *session.Manager,
	fs *vfs.VFS,
	_ *mcp.Manager,
	_ *lsp.Manager,
) *Manager {
	return &Manager{
		log:     log,
		cfg:     cfg,
		bus:     bus,
		fs:      fs,
		plugins: make(map[string]*PluginInstance),
		clients: make(map[string]*goplugin.Client),
		exclude: make(map[string]bool),
	}
}

func (m *Manager) SetExcludedPlugins(exclude []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.exclude = make(map[string]bool)
	for _, name := range exclude {
		m.exclude[name] = true
	}
}

func (m *Manager) LoadPlugins() error {
	pluginDir, err := GetPluginDir()
	if err != nil {
		return fmt.Errorf("get plugin dir: %w", err)
	}

	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return fmt.Errorf("create plugin dir: %w", err)
	}

	pluginPaths, err := scanPlugins(pluginDir)
	if err != nil {
		return fmt.Errorf("scan plugins: %w", err)
	}

	m.log.Finfo("plugin: found %d plugins in %s", len(pluginPaths), pluginDir)

	for _, path := range pluginPaths {
		if err := m.loadPlugin(path); err != nil {
			m.log.Fwarn("plugin: failed to load %s: %v", filepath.Base(path), err)
			continue
		}
	}

	return nil
}

func (m *Manager) loadPlugin(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := filepath.Base(path)

	if m.exclude[name] {
		m.log.Finfo("plugin: skipping excluded plugin %s", name)
		return nil
	}

	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin %s already loaded", name)
	}

	m.log.Finfo("plugin: loading %s", name)

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  pkgplugin.Handshake,
		Plugins:          pkgplugin.PluginMap,
		Cmd:              pluginCmd(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolNetRPC},
		Logger: hclog.New(&hclog.LoggerOptions{
			Output: &logWriter{log: m.log},
			Level:  hclog.Info,
		}),
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return fmt.Errorf("connect to plugin: %w", err)
	}

	raw, err := rpcClient.Dispense("pegg")
	if err != nil {
		client.Kill()
		return fmt.Errorf("dispense plugin: %w", err)
	}

	pluginImpl, ok := raw.(pkgplugin.Plugin)
	if !ok {
		client.Kill()
		return fmt.Errorf("plugin does not implement Plugin interface")
	}

	capabilities := pluginImpl.Capabilities()
	instance := &PluginInstance{
		Name:         pluginImpl.Name(),
		Path:         path,
		Version:      pluginImpl.Version(),
		Description:  pluginImpl.Description(),
		Client:       client,
		Plugin:       pluginImpl,
		Capabilities: capabilities,
	}
	if instance.Name == "" {
		instance.Name = name
	}
	if m.exclude[instance.Name] {
		client.Kill()
		m.log.Finfo("plugin: skipping excluded plugin %s", instance.Name)
		return nil
	}

	configJSON, err := json.Marshal(m.cfg)
	if err != nil {
		client.Kill()
		return fmt.Errorf("marshal config: %w", err)
	}
	var options json.RawMessage
	if m.cfg != nil {
		if raw, ok := m.cfg.Plugins.Options[instance.Name]; ok {
			options = raw
		} else if raw, ok := m.cfg.Plugins.Options[name]; ok {
			options = raw
		}
	}
	deps := pkgplugin.Deps{
		ConfigJSON: configJSON,
		Options:    options,
		Workdir:    m.workspaceRoot(),
		PluginPath: path,
	}
	if err := pluginImpl.Boot(deps); err != nil {
		client.Kill()
		return fmt.Errorf("boot plugin: %w", err)
	}

	m.captureCapabilities(instance)

	m.plugins[instance.Name] = instance
	m.clients[instance.Name] = client
	m.registerEventSubscriptions(instance)

	m.log.Finfo("plugin: loaded %s v%s (%s)", instance.Name, instance.Version, instance.Description)

	return nil
}

func (m *Manager) captureCapabilities(instance *PluginInstance) {
	caps := instance.Capabilities
	if pkgplugin.Supports(caps, pkgplugin.CapabilityTools) {
		if tools, err := instance.Plugin.Tools(); err != nil {
			m.log.Fwarn("plugin: %s: tools: %v", instance.Name, err)
		} else {
			instance.Tools = tools
		}
	}
	if pkgplugin.Supports(caps, pkgplugin.CapabilitySkills) {
		if skills, err := instance.Plugin.Skills(); err != nil {
			m.log.Fwarn("plugin: %s: skills: %v", instance.Name, err)
		} else {
			instance.Skills = skills
		}
	}
	if pkgplugin.Supports(caps, pkgplugin.CapabilityPermissions) {
		if rules, err := instance.Plugin.PermissionRules(); err != nil {
			m.log.Fwarn("plugin: %s: permission rules: %v", instance.Name, err)
		} else {
			instance.PermissionRules = rules
		}
	}
	if pkgplugin.Supports(caps, pkgplugin.CapabilityCommands) {
		if commands, err := instance.Plugin.Commands(); err != nil {
			m.log.Fwarn("plugin: %s: commands: %v", instance.Name, err)
		} else {
			instance.Commands = commands
		}
	}

	m.registerToolSpecs(instance)
	m.registerSkillSpecs(instance)
	m.registerPermissionRules(instance)
	if len(instance.Commands) > 0 {
		for _, spec := range instance.Commands {
			m.commands = append(m.commands, PluginCommand{Plugin: instance.Name, Spec: spec})
		}
	}
}

func (m *Manager) registerEventSubscriptions(instance *PluginInstance) {
	if m.bus == nil || !pkgplugin.Supports(instance.Capabilities, pkgplugin.CapabilityEvents) {
		return
	}
	topics, err := instance.Plugin.EventSubscriptions()
	if err != nil {
		m.log.Fwarn("plugin: %s: event subscriptions: %v", instance.Name, err)
		return
	}
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" {
			continue
		}
		topic := topic
		handler := func(payload interface{}) {
			raw, err := json.Marshal(payload)
			if err != nil {
				m.log.Fwarn("plugin: %s: marshal event %s: %v", instance.Name, topic, err)
				return
			}
			m.forwardEvent(instance.Name, pkgplugin.Event{Topic: topic, Payload: raw})
		}
		if err := m.bus.SubscribeAsync(topic, handler, false); err != nil {
			m.log.Fwarn("plugin: %s: subscribe %s: %v", instance.Name, topic, err)
			continue
		}
		m.eventUnsubs = append(m.eventUnsubs, func() error {
			return m.bus.Unsubscribe(topic, handler)
		})
		m.log.Finfo("plugin: %s subscribed to event %s", instance.Name, topic)
	}
}

func (m *Manager) forwardEvent(pluginName string, event pkgplugin.Event) {
	instance, ok := m.GetPlugin(pluginName)
	if !ok {
		return
	}
	instance.eventMu.Lock()
	defer instance.eventMu.Unlock()
	if err := instance.Plugin.OnEvent(event); err != nil {
		m.log.Fwarn("plugin: %s: event %s: %v", pluginName, event.Topic, err)
	}
}

func (m *Manager) registerToolSpecs(instance *PluginInstance) {
	byBundle := make(map[string][]tool.Tool)
	for _, spec := range instance.Tools {
		if strings.TrimSpace(spec.Name) == "" {
			continue
		}
		bundleName := strings.TrimSpace(spec.Bundle)
		if bundleName == "" {
			bundleName = instance.Name
		}
		adapter := &pluginTool{
			m:          m,
			pluginName: instance.Name,
			spec:       spec,
			fs:         m.fs,
		}
		if _, exists := agenttools.Get(spec.Name); !exists {
			if err := agenttools.Register(adapter); err != nil {
				m.log.Fwarn("plugin: %s: register global tool %s: %v", instance.Name, spec.Name, err)
			}
		}
		byBundle[bundleName] = append(byBundle[bundleName], adapter)
	}
	for bundleName, ts := range byBundle {
		if err := bundle.RegisterTools(bundleName, ts); err != nil {
			m.log.Fwarn("plugin: %s: register bundle %s: %v", instance.Name, bundleName, err)
		}
	}
}

func (m *Manager) registerSkillSpecs(instance *PluginInstance) {
	for _, spec := range instance.Skills {
		s := &skill.Skill{
			Name:         spec.Name,
			Description:  spec.Description,
			WhenToUse:    spec.WhenToUse,
			Instructions: spec.Instructions,
			ArgumentHint: spec.ArgumentHint,
			Arguments:    spec.Arguments,
			AllowedTools: spec.AllowedTools,
			ToolBundles:  spec.ToolBundles,
			Context:      spec.Context,
			Source:       "plugin:" + instance.Name,
			Path:         spec.Path,
		}
		if s.Context == "" {
			s.Context = "inline"
		}
		if s.Path == "" {
			s.Path = instance.Path
		}
		if err := skill.RegisterDynamic(s); err != nil {
			m.log.Fwarn("plugin: %s: register skill %s: %v", instance.Name, spec.Name, err)
		}
	}
}

func (m *Manager) registerPermissionRules(instance *PluginInstance) {
	if m.cfg == nil {
		return
	}
	rules := make(map[string]string, len(instance.Tools)+len(instance.PermissionRules))
	for _, spec := range instance.Tools {
		if spec.Name == "" {
			continue
		}
		if spec.ReadOnly {
			rules[spec.Name] = "allow"
		} else {
			rules[spec.Name] = "semi-judge"
		}
	}
	for name, mode := range instance.PermissionRules {
		if name != "" && mode != "" {
			rules[name] = mode
		}
	}
	if len(rules) == 0 {
		return
	}
	if m.cfg.Permission == nil {
		m.cfg.Permission = &config.PermissionConfig{Default: "semi-ask", Rules: map[string]string{}}
	}
	if m.cfg.Permission.Rules == nil {
		m.cfg.Permission.Rules = map[string]string{}
	}
	for name, mode := range rules {
		if _, exists := m.cfg.Permission.Rules[name]; exists {
			continue
		}
		m.cfg.Permission.Rules[name] = mode
	}
}

func (m *Manager) workspaceRoot() string {
	if m.fs != nil {
		return m.fs.Root()
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

func (m *Manager) ExecuteTool(ctx context.Context, pluginName, toolName, args string) (*pkgplugin.ToolResult, error) {
	instance, ok := m.GetPlugin(pluginName)
	if !ok {
		return nil, fmt.Errorf("plugin %q is not loaded", pluginName)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	instance.execMu.Lock()
	defer instance.execMu.Unlock()

	res, err := instance.Plugin.ExecuteTool(pkgplugin.ToolRequest{
		Name:      toolName,
		Args:      args,
		TimeoutMS: remainingMillis(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("plugin %s tool %s: %w", pluginName, toolName, err)
	}
	if res == nil {
		res = &pkgplugin.ToolResult{}
	}
	return res, nil
}

func (m *Manager) RunCommand(ctx context.Context, pluginName string, path []string, args []string, flags map[string]string) (string, error) {
	instance, ok := m.GetPlugin(pluginName)
	if !ok {
		return "", fmt.Errorf("plugin %q is not loaded", pluginName)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	instance.execMu.Lock()
	defer instance.execMu.Unlock()

	res, err := instance.Plugin.RunCommand(pkgplugin.CommandRequest{
		Path:  path,
		Args:  args,
		Flags: flags,
	})
	if err != nil {
		return "", fmt.Errorf("plugin %s command %s: %w", pluginName, strings.Join(path, " "), err)
	}
	if res == nil {
		return "", nil
	}
	return res.Output, nil
}

func remainingMillis(ctx context.Context) int64 {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	ms := time.Until(deadline).Milliseconds()
	if ms < 1 {
		return 1
	}
	return ms
}

func (m *Manager) GetPlugin(name string) (*PluginInstance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[name]
	return p, ok
}

func (m *Manager) ListPlugins() []*PluginInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]*PluginInstance, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}

	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})

	return plugins
}

func (m *Manager) Commands() []PluginCommand {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]PluginCommand, len(m.commands))
	copy(out, m.commands)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Plugin != out[j].Plugin {
			return out[i].Plugin < out[j].Plugin
		}
		return out[i].Spec.Use < out[j].Spec.Use
	})
	return out
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, unsub := range m.eventUnsubs {
		_ = unsub()
	}
	m.eventUnsubs = nil

	for name, instance := range m.plugins {
		done := make(chan struct{})
		go func(p pkgplugin.Plugin) {
			_ = p.Close()
			close(done)
		}(instance.Plugin)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			m.log.Fwarn("plugin: %s: close timed out", name)
		}
	}

	for name, client := range m.clients {
		m.log.Finfo("plugin: shutting down %s", name)
		client.Kill()
	}

	m.plugins = make(map[string]*PluginInstance)
	m.clients = make(map[string]*goplugin.Client)
	m.commands = nil
}

func GetPluginDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".pegg", "plugins"), nil
}

func pluginCmd(path string) *exec.Cmd {
	return exec.Command(path)
}
