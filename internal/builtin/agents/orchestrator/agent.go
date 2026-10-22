package orchestrator

import (
	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/agents"
	_ "github.com/peggco/pegg/internal/builtin/middlewares"
	"github.com/peggco/pegg/internal/builtin/tools/file"
	"github.com/peggco/pegg/internal/builtin/tools/plan"
	"github.com/peggco/pegg/internal/vfs"
)

func Register(fs *vfs.VFS) error {
	return agents.Register(func() (*agent.Agent, error) {
		return newOrchestratorAgent(fs)
	})
}

const (
	agentTemperature   = 0.2
	agentTopP          = 0.9
	agentMaxTokens     = 4096
	agentMaxIterations = agent.UnlimitedIterations
)

func newOrchestratorAgent(fs *vfs.VFS) (*agent.Agent, error) {
	main := agent.New("orchestrator",
		agent.WithSystemPromptFn(func(providerID, modelID string) string {
			sys, err := generateOrchestratorPrompt(providerID, modelID)
			if err != nil {
				return ""
			}
			return sys
		}),
		agent.WithTools(file.Tools(fs)...),
		agent.WithToolNames("askuserquestion", "bash", "task", "taskstatus", "todoread", "todowrite", "webfetch", "websearch", "loadskill", "enterplanmode", "exitplanmode", "mem-search", "mem-read", "schedule"),
		agent.WithMiddlewareNames("loop-detector", "retry", "permission", "compaction"),
		agent.WithTemperature(agentTemperature),
		agent.WithTopP(agentTopP),
		agent.WithMaxTokens(agentMaxTokens),
		agent.WithMaxIterations(agentMaxIterations),
		agent.WithModeController(func(a *agent.Agent, m agent.Mode) {
			plan.ApplyMode(a, fs, m)
		}),
	)

	return main, nil
}
