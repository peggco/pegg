package plugin

import (
	"os/exec"
	"strings"

	"github.com/peggco/pegg/internal/core/cache"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/lsp"
	"github.com/peggco/pegg/internal/mcp"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/vfs"
)

func Module(
	cfg *config.Config,
	bus event.Bus,
	log *logger.Logger,
	cacheStore cache.Cache,
	llmMgr *llm.Manager,
	sess *session.Manager,
	fs *vfs.VFS,
	mcpMgr *mcp.Manager,
	lspMgr *lsp.Manager,
) (*Manager, error) {
	pluginCfg := cfg.Plugins
	if !pluginCfg.Enabled {
		log.Info("plugin: system disabled")
		return NewManager(log, cfg, bus, cacheStore, llmMgr, sess, fs, mcpMgr, lspMgr), nil
	}

	mgr := NewManager(log, cfg, bus, cacheStore, llmMgr, sess, fs, mcpMgr, lspMgr)
	mgr.SetExcludedPlugins(pluginCfg.Exclude)
	log.Fdebug("plugin: loading from %s (excluded: %s, gh: %s)",
		pluginDirForLog(), excludedForLog(pluginCfg.Exclude), ghForLog())

	if err := mgr.LoadPlugins(); err != nil {
		log.Fwarn("plugin: failed to load plugins: %v", err)
	}

	plugins := mgr.ListPlugins()
	if len(plugins) > 0 {
		log.Finfo("plugin: loaded %d plugins", len(plugins))
		for _, p := range plugins {
			log.Finfo("plugin:   - %s v%s", p.Name, p.Version)
		}
	}

	return mgr, nil
}

func pluginDirForLog() string {
	dir, err := GetPluginDir()
	if err != nil {
		return "(unknown)"
	}
	return dir
}

func excludedForLog(exclude []string) string {
	if len(exclude) == 0 {
		return "none"
	}
	return strings.Join(exclude, ",")
}

func ghForLog() string {
	path, err := exec.LookPath("gh")
	if err != nil {
		return "not found on PATH"
	}
	return path
}
