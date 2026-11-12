package skills

import (
	"github.com/peggco/pegg/internal/agent/prompt"
)

func All() map[string]func() *prompt.Prompt {
	return map[string]func() *prompt.Prompt{
		"batch":       BatchSkill,
		"review":      ReviewSkill,
		"init":        InitSkill,
		"orchestrate": OrchestrateSkill,
		"rule":        RuleSkill,
		"skillify":    SkillifySkill,
	}
}
