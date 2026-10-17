package agent

type Mode string

const (
	ModeBuild Mode = "build"
	ModePlan  Mode = "plan"
	ModeAuto  Mode = "auto"
)

const (
	ToolEnterPlanMode = "enterplanmode"
	ToolExitPlanMode  = "exitplanmode"
)

func (m Mode) Valid() bool {
	switch m {
	case ModeBuild, ModePlan, ModeAuto:
		return true
	}
	return false
}

func NormalizeMode(m Mode) Mode {
	if m.Valid() {
		return m
	}
	return ModeAuto
}

func isPlanTool(name string) bool {
	return name == ToolEnterPlanMode || name == ToolExitPlanMode
}

type ModeController func(a *Agent, m Mode)
