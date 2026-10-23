package planner

import (
	"fmt"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/agents"
	_ "github.com/peggco/pegg/internal/builtin/middlewares"
	"github.com/peggco/pegg/internal/builtin/tools/file"
	"github.com/peggco/pegg/internal/builtin/tools/plan"
	"github.com/peggco/pegg/internal/vfs"
)

const (
	plansScope = vfs.PlansDir

	agentTemperature   = 0.4
	agentTopP          = 0.95
	agentMaxTokens     = 8192
	agentMaxIterations = 150
)

func Register(fs *vfs.VFS) error {
	return agents.Register(func() (*agent.Agent, error) {
		return newPlannerAgent(fs)
	})
}

func newPlannerAgent(fs *vfs.VFS) (*agent.Agent, error) {
	plans, err := fs.WriteScope(plansScope)
	if err != nil {
		return nil, fmt.Errorf("planner: scope file tools to %s: %w", plansScope, err)
	}

	main := agent.New("planner",
		agent.WithSystemPromptFn(func(providerID, modelID string) string {
			sys, err := generatePlannerPrompt(providerID, modelID)
			if err != nil {
				return ""
			}
			return sys
		}),
		agent.WithTools(file.Tools(plans)...),
		agent.WithToolNames("bash", "webfetch", "websearch", "todoread", "todowrite", "mem-search", "mem-read"),
		agent.WithMiddlewareNames("loop-detector", "retry", "permission", "compaction"),
		agent.WithTemperature(agentTemperature),
		agent.WithTopP(agentTopP),
		agent.WithMaxTokens(agentMaxTokens),
		agent.WithMaxIterations(agentMaxIterations),
	)
	main.AttachReminder(plan.PlanModeReminder(plan.PlanExitNone))

	return main, nil
}
