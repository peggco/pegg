package plugin

import (
	"context"
	"encoding/json"
	"errors"
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
	corehook "github.com/peggco/pegg/internal/core/hook"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/lsp"
	"github.com/peggco/pegg/internal/mcp"
	"github.com/peggco/pegg/internal/plugin/bundle"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/skill"
	"github.com/peggco/pegg/internal/tui/ext"
	"github.com/peggco/pegg/internal/vfs"
)

type Manager struct {
	mu          sync.RWMutex
	rootMu      sync.RWMutex
	root        string
	log         *logger.Logger
	cfg         *config.Config
	bus         event.Bus
	fs          *vfs.VFS
	plugins     map[string]*PluginInstance
	clients     map[string]*goplugin.Client
	exclude     map[string]bool
	commands    []PluginCommand
	eventUnsubs []func() error
	registry    *corehook.Registry

	hookMu    sync.Mutex
	hookDepth map[string]int
}

func (m *Manager) SetWorkspaceRoot(root string) {
	m.rootMu.Lock()
	m.root = strings.TrimSpace(root)
	m.rootMu.Unlock()
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
	TUIPages        []pkgplugin.TUIPage
	Hooks           []pkgplugin.HookSpec

	execMu  sync.Mutex
	eventMu sync.Mutex
}

type PluginCommand struct {
	Plugin string
	Spec   pkgplugin.CommandSpec
}

type PluginHookSpec struct {
	Plugin      string
	Name        string
	Kind        pkgplugin.HookKind
	Priority    int
	Description string
}

type HookOutcome struct {
	Value   json.RawMessage   `json:"value"`
	Outputs []json.RawMessage `json:"outputs"`
	Errors  []string          `json:"errors"`
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
		log:       log,
		cfg:       cfg,
		bus:       bus,
		fs:        fs,
		plugins:   make(map[string]*PluginInstance),
		clients:   make(map[string]*goplugin.Client),
		exclude:   make(map[string]bool),
		registry:  corehook.Default().Registry(),
		hookDepth: make(map[string]int),
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

const pluginLoadWorkers = 8

var errPluginSkipped = errors.New("plugin skipped")

type pluginLoadResult struct {
	instance *PluginInstance
	err      error
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

	results := make([]pluginLoadResult, len(pluginPaths))
	sem := make(chan struct{}, pluginLoadWorkers)
	var wg sync.WaitGroup
	for i, path := range pluginPaths {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			instance, err := m.connectPlugin(path)
			results[i] = pluginLoadResult{instance: instance, err: err}
		}(i, path)
	}
	wg.Wait()

	for _, result := range results {
		if result.err != nil {
			if !errors.Is(result.err, errPluginSkipped) {
				m.log.Fwarn("plugin: %v", result.err)
			}
			continue
		}
		if err := m.registerPlugin(result.instance); err != nil {
			if errors.Is(err, errPluginSkipped) {
				continue
			}
			m.log.Fwarn("plugin: failed to register %s: %v", result.instance.Name, err)
			if result.instance.Client != nil {
				result.instance.Client.Kill()
			}
		}
	}

	return nil
}

func (m *Manager) pluginExcluded(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.exclude[name]
}

// connectPlugin starts one plugin process, performs the handshake and Boot, and
// fetches its capability metadata. It does not touch shared manager state, so
// plugins can be connected concurrently.
func (m *Manager) connectPlugin(path string) (*PluginInstance, error) {
	name := filepath.Base(path)
	if m.pluginExcluded(name) {
		m.log.Finfo("plugin: skipping excluded plugin %s", name)
		return nil, errPluginSkipped
	}

	m.log.Finfo("plugin: loading %s", name)
	start := time.Now()

	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  pkgplugin.Handshake,
		Plugins:          pkgplugin.NewClientPluginMap(m.dispatchHook),
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
		return nil, fmt.Errorf("connect to plugin %s: %w", name, err)
	}

	raw, err := rpcClient.Dispense("pegg")
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("dispense plugin %s: %w", name, err)
	}

	pluginImpl, ok := raw.(pkgplugin.Plugin)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("plugin %s does not implement Plugin interface", name)
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
	if m.pluginExcluded(instance.Name) {
		client.Kill()
		m.log.Finfo("plugin: skipping excluded plugin %s", instance.Name)
		return nil, errPluginSkipped
	}

	configJSON, err := json.Marshal(m.cfg)
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("marshal config for %s: %w", instance.Name, err)
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
		return nil, fmt.Errorf("boot plugin %s: %w", instance.Name, err)
	}

	m.fetchCapabilities(instance)
	m.log.Finfo("plugin: %s ready in %s", instance.Name, time.Since(start).Round(time.Millisecond))
	return instance, nil
}

// registerPlugin publishes a connected plugin into the manager and global
// registries. It must run serially so tool/skill/page registration stays
// deterministic.
func (m *Manager) registerPlugin(instance *PluginInstance) error {
	if instance == nil {
		return fmt.Errorf("plugin is nil")
	}

	m.mu.Lock()

	if m.exclude[instance.Name] {
		m.mu.Unlock()
		if instance.Client != nil {
			instance.Client.Kill()
		}
		return errPluginSkipped
	}
	if _, exists := m.plugins[instance.Name]; exists {
		m.mu.Unlock()
		return fmt.Errorf("plugin %s already loaded", instance.Name)
	}

	m.plugins[instance.Name] = instance
	m.clients[instance.Name] = instance.Client
	m.registerCapabilities(instance)
	m.registerEventSubscriptions(instance)
	m.registerHooks(instance)
	m.mu.Unlock()

	if pkgplugin.Supports(instance.Capabilities, pkgplugin.CapabilityTUI) {
		ext.Register(func() []pkgplugin.TUIPage { return m.tuiPagesFor(instance.Name) })
	}

	m.log.Finfo("plugin: loaded %s v%s (%s)", instance.Name, instance.Version, instance.Description)
	return nil
}

func (m *Manager) fetchCapabilities(instance *PluginInstance) {
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
	if pkgplugin.Supports(caps, pkgplugin.CapabilityTUI) {
		if pages, err := instance.Plugin.TUIPages(); err != nil {
			m.log.Fwarn("plugin: %s: tui pages: %v", instance.Name, err)
		} else {
			for i := range pages {
				pages[i].Owner = instance.Name
			}
			instance.TUIPages = pages
		}
	}
	if caps != nil && pkgplugin.Supports(caps, pkgplugin.CapabilityHooks) {
		if hooks, err := instance.Plugin.Hooks(); err != nil {
			m.log.Fwarn("plugin: %s: hooks: %v", instance.Name, err)
		} else {
			instance.Hooks = normalizeHookSpecs(hooks)
		}
	}
}

func normalizeHookSpecs(specs []pkgplugin.HookSpec) []pkgplugin.HookSpec {
	out := make([]pkgplugin.HookSpec, 0, len(specs))
	for _, spec := range specs {
		spec.Name = strings.TrimSpace(spec.Name)
		if spec.Name == "" {
			continue
		}
		if spec.Kind != pkgplugin.HookAction {
			spec.Kind = pkgplugin.HookFilter
		}
		if spec.Priority == 0 {
			spec.Priority = 10
		}
		out = append(out, spec)
	}
	return out
}

func (m *Manager) registerCapabilities(instance *PluginInstance) {
	m.registerToolSpecs(instance)
	m.registerSkillSpecs(instance)
	m.registerPermissionRules(instance)
	if len(instance.Commands) > 0 {
		for _, spec := range instance.Commands {
			m.commands = append(m.commands, PluginCommand{Plugin: instance.Name, Spec: spec})
		}
	}
}

// captureCapabilities is kept for callers that register a plugin by hand
// (tests). Production loading uses fetchCapabilities + registerPlugin.
func (m *Manager) captureCapabilities(instance *PluginInstance) {
	m.fetchCapabilities(instance)
	m.mu.Lock()
	m.registerCapabilities(instance)
	m.mu.Unlock()
	if pkgplugin.Supports(instance.Capabilities, pkgplugin.CapabilityTUI) {
		ext.Register(func() []pkgplugin.TUIPage { return m.tuiPagesFor(instance.Name) })
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

func (m *Manager) registerHooks(instance *PluginInstance) {
	for _, spec := range instance.Hooks {
		name := spec.Name
		kind := spec.Kind
		callback := func(ctx context.Context, value any) (any, error) {
			raw, err := json.Marshal(value)
			if err != nil {
				return nil, fmt.Errorf("encode %s payload: %w", name, err)
			}
			timeoutMS, _ := ctx.Value(hookTimeoutKey{}).(int64)
			if timeoutMS <= 0 {
				timeoutMS = remainingMillis(ctx)
			}
			req := pkgplugin.HookRequest{
				Name:      name,
				Kind:      kind,
				Value:     raw,
				TimeoutMS: timeoutMS,
			}
			if !lockHookExec(ctx, instance) {
				return nil, fmt.Errorf("%s: hook %s busy: %w", instance.Name, name, ctx.Err())
			}
			result, err := instance.Plugin.ExecuteHook(req)
			instance.execMu.Unlock()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", instance.Name, err)
			}
			if result == nil || len(result.Value) == 0 {
				return nil, nil
			}
			if kind == pkgplugin.HookFilter && !result.Handled {
				return nil, nil
			}
			var out any
			if err := json.Unmarshal(result.Value, &out); err != nil {
				return nil, fmt.Errorf("%s: decode %s result: %w", instance.Name, name, err)
			}
			return out, nil
		}
		if kind == pkgplugin.HookAction {
			m.registry.AddActionFor(instance.Name, name, spec.Priority, callback)
		} else {
			m.registry.AddFilterFor(instance.Name, name, spec.Priority, callback)
		}
	}
}

func lockHookExec(ctx context.Context, instance *PluginInstance) bool {
	for {
		if instance.execMu.TryLock() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(2 * time.Millisecond):
		}
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
		if spec.AlwaysAvailable {
			if _, exists := agenttools.Get(spec.Name); !exists {
				if err := agenttools.Register(adapter); err != nil {
					m.log.Fwarn("plugin: %s: register always-available tool %s: %v", instance.Name, spec.Name, err)
				}
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
	m.rootMu.RLock()
	root := m.root
	m.rootMu.RUnlock()
	if root != "" {
		return root
	}
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

func (m *Manager) tuiPagesFor(name string) []pkgplugin.TUIPage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	instance, ok := m.plugins[name]
	if !ok {
		return nil
	}
	out := make([]pkgplugin.TUIPage, len(instance.TUIPages))
	copy(out, instance.TUIPages)
	return out
}

func (m *Manager) Pages(owner string) []pkgplugin.TUIPage {
	return m.tuiPagesFor(owner)
}

func (m *Manager) PageOwners() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	owners := make([]string, 0, len(m.plugins))
	for name, instance := range m.plugins {
		if pkgplugin.Supports(instance.Capabilities, pkgplugin.CapabilityTUI) {
			owners = append(owners, name)
		}
	}
	sort.Strings(owners)
	return owners
}

func (m *Manager) RefreshTUIPages(ctx context.Context, name string) ([]pkgplugin.TUIPage, error) {
	instance, ok := m.GetPlugin(name)
	if !ok {
		return nil, fmt.Errorf("plugin %q is not loaded", name)
	}
	if !pkgplugin.Supports(instance.Capabilities, pkgplugin.CapabilityTUI) {
		return nil, fmt.Errorf("plugin %q does not support tui", name)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	instance.execMu.Lock()
	defer instance.execMu.Unlock()

	pages, err := instance.Plugin.TUIPages()
	if err != nil {
		return nil, fmt.Errorf("plugin %s tui pages: %w", name, err)
	}
	for i := range pages {
		pages[i].Owner = name
	}
	m.mu.Lock()
	instance.TUIPages = pages
	m.mu.Unlock()

	out := make([]pkgplugin.TUIPage, len(pages))
	copy(out, pages)
	return out, nil
}

func (m *Manager) ExecuteTUIAction(ctx context.Context, name string, req pkgplugin.TUIActionRequest) (*pkgplugin.TUIActionResult, error) {
	instance, ok := m.GetPlugin(name)
	if !ok {
		return nil, fmt.Errorf("plugin %q is not loaded", name)
	}
	if !pkgplugin.Supports(instance.Capabilities, pkgplugin.CapabilityTUI) {
		return nil, fmt.Errorf("plugin %q does not support tui", name)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	instance.execMu.Lock()
	defer instance.execMu.Unlock()

	res, err := instance.Plugin.ExecuteTUIAction(req)
	if err != nil {
		return nil, fmt.Errorf("plugin %s tui action %s: %w", name, req.ActionID, err)
	}
	if res == nil {
		res = &pkgplugin.TUIActionResult{}
	}
	if res.Page != nil {
		res.Page.Owner = name
		m.mu.Lock()
		for i := range instance.TUIPages {
			if instance.TUIPages[i].ID == res.Page.ID {
				instance.TUIPages[i] = *res.Page
				break
			}
		}
		m.mu.Unlock()
	}
	return res, nil
}

func (m *Manager) HookSpecs() []PluginHookSpec {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]PluginHookSpec, 0)
	for _, instance := range m.plugins {
		for _, spec := range instance.Hooks {
			out = append(out, PluginHookSpec{
				Plugin:      instance.Name,
				Name:        spec.Name,
				Kind:        spec.Kind,
				Priority:    spec.Priority,
				Description: spec.Description,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].Plugin < out[j].Plugin
	})
	return out
}

func (m *Manager) ApplyHook(ctx context.Context, name string, kind pkgplugin.HookKind, value json.RawMessage, timeout time.Duration) (HookOutcome, error) {
	return m.ApplyHookFrom(ctx, name, kind, value, timeout, "")
}

func (m *Manager) ApplyHookFrom(ctx context.Context, name string, kind pkgplugin.HookKind, value json.RawMessage, timeout time.Duration, skipPlugin string) (HookOutcome, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if kind == "" {
		kind = pkgplugin.HookFilter
	}
	if kind != pkgplugin.HookFilter && kind != pkgplugin.HookAction {
		return HookOutcome{}, fmt.Errorf("hook %q: unknown kind %q", name, kind)
	}
	if err := ctx.Err(); err != nil {
		return HookOutcome{}, err
	}
	registryKind := corehook.Kind(kind)
	if !m.registry.HasFor(name, registryKind, skipPlugin) {
		return HookOutcome{}, fmt.Errorf("hook %q (%s): no handler registered", name, kind)
	}
	if timeout > 0 {
		ctx = context.WithValue(ctx, hookTimeoutKey{}, timeout.Milliseconds())
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	var payload any
	if len(value) > 0 {
		if err := json.Unmarshal(value, &payload); err != nil {
			return HookOutcome{}, fmt.Errorf("hook %q: decode payload: %w", name, err)
		}
	}
	outcome := HookOutcome{Value: value}
	switch kind {
	case pkgplugin.HookFilter:
		next, errs := m.registry.ApplyFiltersFrom(ctx, name, payload, skipPlugin)
		if raw, err := json.Marshal(next); err == nil {
			outcome.Value = raw
		}
		outcome.Errors = hookErrors(errs)
	case pkgplugin.HookAction:
		outputs, errs := m.registry.DoActionFrom(ctx, name, payload, skipPlugin)
		for _, output := range outputs {
			raw, err := json.Marshal(output)
			if err != nil {
				outcome.Errors = append(outcome.Errors, err.Error())
				continue
			}
			outcome.Outputs = append(outcome.Outputs, raw)
		}
		outcome.Errors = append(outcome.Errors, hookErrors(errs)...)
	}
	return outcome, ctx.Err()
}

const maxHookDepth = 4

func (m *Manager) dispatchHook(req pkgplugin.HookRequest) (*pkgplugin.HookResult, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("hook name is required")
	}
	origin := strings.TrimSpace(req.Origin)
	key := origin + "\x00" + name
	if !m.enterHook(key) {
		return nil, fmt.Errorf("hook %q recursion limit reached", name)
	}
	defer m.leaveHook(key)

	timeout := time.Duration(req.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	outcome, err := m.ApplyHookFrom(context.Background(), name, req.Kind, req.Value, timeout, origin)
	if err != nil {
		return nil, err
	}
	return &pkgplugin.HookResult{
		Value:   outcome.Value,
		Handled: true,
		Outputs: outcome.Outputs,
		Errors:  outcome.Errors,
	}, nil
}

func (m *Manager) enterHook(key string) bool {
	m.hookMu.Lock()
	defer m.hookMu.Unlock()
	if m.hookDepth[key] >= maxHookDepth {
		return false
	}
	m.hookDepth[key]++
	return true
}

func (m *Manager) leaveHook(key string) {
	m.hookMu.Lock()
	defer m.hookMu.Unlock()
	m.hookDepth[key]--
	if m.hookDepth[key] <= 0 {
		delete(m.hookDepth, key)
	}
}

type hookTimeoutKey struct{}

func hookErrors(errs []error) []string {
	out := make([]string, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			out = append(out, err.Error())
		}
	}
	return out
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

func (m *Manager) ToolNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	seen := map[string]bool{}
	var names []string
	for _, p := range m.plugins {
		for _, spec := range p.Tools {
			if spec.Name == "" || seen[spec.Name] {
				continue
			}
			seen[spec.Name] = true
			names = append(names, spec.Name)
		}
	}
	sort.Strings(names)
	return names
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
		m.registry.RemoveOwner(name)
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
	ext.Reset()
}

func GetPluginDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".pegg", "plugins"), nil
}

func pluginCmd(path string) *exec.Cmd {
	cmd := exec.Command(path)
	configurePluginCmd(cmd)
	return cmd
}
