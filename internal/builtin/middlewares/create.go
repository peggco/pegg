package middlewares

import (
	"github.com/peggco/pegg/internal/agent/middleware"
	"github.com/peggco/pegg/internal/builtin/middlewares/compaction"
	"github.com/peggco/pegg/internal/builtin/middlewares/permission"
	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/decision"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
	"github.com/peggco/pegg/internal/vfs"
)

type Deps struct {
	Config   *config.Config
	LLM      *llm.Manager
	Decision *decision.Manager
	Sessions *session.Manager
}

func Create(fs *vfs.VFS, deps Deps) {
	middleware.Default().Register("loop-detector", NewLoopDetector())
	middleware.Default().Register("redaction", NewRedaction())
	middleware.Default().Register("retry", NewRetry())
	if stall := stallFromConfig(deps.Config); stall != nil {
		middleware.Default().Register("stall", stall)
	}
	var permCfg *config.PermissionConfig
	if deps.Config != nil {
		permCfg = deps.Config.Permission
	}
	perm := permission.New(permission.Deps{
		Config:   permCfg,
		LLM:      deps.LLM,
		Decision: deps.Decision,
	})
	middleware.Default().Register("permission", perm)
	if fs != nil {
		fs.OnAccessCheck(perm.AccessChecker)
	}
	var compCfg *config.CompactionConfig
	if deps.Config != nil {
		compCfg = deps.Config.Compaction
	}
	middleware.Default().Register("compaction", compaction.New(compaction.Deps{
		Config: compCfg,
		LLM:    deps.LLM,
	}))
}

func stallFromConfig(cfg *config.Config) *Stall {
	var sc *config.StallConfig
	if cfg != nil {
		sc = cfg.Stall
	}
	if sc != nil && !sc.EnabledValue() {
		return nil
	}
	opts := []StallOption{WithStallTimeout(sc.TimeoutValue())}
	if sc != nil {
		opts = append(opts, WithStallMaxRetries(sc.MaxRetries))
	}
	return NewStall(opts...)
}
