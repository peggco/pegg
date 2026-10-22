package agents

import (
	"github.com/peggco/pegg/internal/builtin/agents/developer"
	"github.com/peggco/pegg/internal/builtin/agents/explorer"
	"github.com/peggco/pegg/internal/builtin/agents/orchestrator"
	"github.com/peggco/pegg/internal/builtin/agents/planner"
	"github.com/peggco/pegg/internal/vfs"
)

func Create(fs *vfs.VFS) {
	if err := explorer.Register(); err != nil {
		fs.Warn("agents: explorer is disabled: %v", err)
	}
	if err := planner.Register(fs); err != nil {
		fs.Warn("agents: planner is disabled: %v", err)
	}
	if err := developer.Register(fs); err != nil {
		fs.Warn("agents: developer is disabled: %v", err)
	}
	if err := orchestrator.Register(fs); err != nil {
		fs.Warn("agents: orchestrator is disabled: %v", err)
	}
}
