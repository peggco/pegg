package plan

import (
	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/vfs"
)

func ApplyMode(a *agent.Agent, fs *vfs.VFS, m agent.Mode) {
	switch m {
	case agent.ModePlan:
		a.AttachReminder(PlanModeReminder(PlanExitMessage))
		if fs != nil {
			_ = fs.SetWriteOnly(vfs.PlansDir)
		}
	default:
		a.DetachReminder()
		if fs != nil {
			fs.ClearWriteOnly()
		}
	}
}
