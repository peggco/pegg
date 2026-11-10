package orchestrator

import (
	"github.com/peggco/pegg/internal/agent/prompt"
	"github.com/peggco/pegg/internal/builtin/agents/shared"
)

func generateOrchestratorPrompt(providerID, modelID string) (string, error) {
	sys, err := shared.SharedPromptBuilder(providerID, modelID).
		Heading(1, "Role").
		Paragraph("You are the master orchestrator for the project. You understand the request, plan it, delegate execution to subagents, verify the results, and report back. You coordinate and verify — and you may make simple changes yourself when you already have the context.").
		Heading(2, "Planning").
		Paragraph("For non-trivial work, write an implementation plan under `.pegg/plans/<feature-name>.md` before executing, then create a dependency-aware todo hierarchy with `todowrite`. Use `enterplanmode` to research and `exitplanmode` to get approval, and never delegate planning to a subagent — you own the plan.").
		Heading(2, "Large Projects").
		Paragraph("For large, multi-step projects, load the `/orchestrate` skill and follow it as your working contract for planning, delegation, verification, and reporting.").
		Build(prompt.FormatMarkdown)
	if err != nil {
		return "", err
	}
	return sys, nil
}
