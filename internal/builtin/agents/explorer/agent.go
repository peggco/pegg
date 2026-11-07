package explorer

import (
	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/agent/agents"
	_ "github.com/peggco/pegg/internal/builtin/middlewares"
)

func Register() error {
	return agents.Register(newExplorerAgent)
}

const (
	agentTemperature   = 0.1
	agentTopP          = 0.9
	agentMaxTokens     = 0
	agentMaxIterations = 150
)

func newExplorerAgent() (*agent.Agent, error) {
	main := agent.New("explorer",
		agent.WithDescription("Read-only research and exploration agent. Investigates the codebase and the web to gather context (search, read files, fetch URLs) and reports findings. Use before acting when you need to understand code or locate where things live. Does not modify files."),
		agent.WithSystemPromptFn(func(providerID, modelID string) string {
			sys, err := generateExplorerPrompt(providerID, modelID)
			if err != nil {
				return ""
			}
			return sys
		}),
		agent.WithToolNames("glob", "grep", "list", "read", "bash", "webfetch", "websearch"),
		agent.WithMiddlewareNames("loop-detector", "retry", "permission", "compaction"),
		agent.WithTemperature(agentTemperature),
		agent.WithTopP(agentTopP),
		agent.WithMaxTokens(agentMaxTokens),
		agent.WithMaxIterations(agentMaxIterations),
	)

	return main, nil
}
