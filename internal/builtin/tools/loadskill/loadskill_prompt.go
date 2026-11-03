package loadskill

import (
	"github.com/peggco/pegg/internal/agent/prompt"
	"github.com/peggco/pegg/internal/skill"
)

func loadSkillPromptBuilder() *prompt.Prompt {
	p := prompt.New().
		Paragraph("Use this tool to load a skill by name and optionally pass arguments to it.").
		Paragraph("If the skill is not loaded, this tool automatically searches the Pegg hub, installs the best exact match into the global skills directory (~/.pegg/skills), and loads it for the current conversation.").
		Paragraph("Usage:").
		List(
			"Check Available Skills first. If a loaded skill matches the task, load it directly.",
			"If the needed skill is missing, use searchskill first, then call loadskill with that result.",
			"Provide the skill name exactly as registered, or pass a hub reference like 'owner/repo/skill' as the name.",
			"If several hub skills share the same name, pass the result's source or full ref to disambiguate.",
			"Optionally pass an arguments map to substitute $arg_name placeholders in the skill content.",
			"If the skill defines 'arguments' in its frontmatter, supply values for those names.",
			"The tool returns the expanded skill content with arguments substituted.",
		).
		Paragraph("Examples:").
		List(
			"loadskill {\"name\": \"review\"} when a listed review skill matches the task.",
			"searchskill {\"query\": \"docx word document\"}, then loadskill {\"name\": \"docx\"} before editing an uploaded .docx file.",
			"loadskill {\"name\": \"anthropics/skills/docx\"} to load a hub result by its full reference.",
			"loadskill {\"name\": \"docx\", \"source\": \"anthropics/skills\"} when multiple hub skills are named docx.",
			"loadskill {\"name\": \"report\", \"arguments\": {\"format\": \"pdf\"}} to fill a skill's $format placeholder.",
		).
		Add(prompt.IfFn(
			func(vars prompt.Vars) bool { return len(skill.List()) > 0 },
			prompt.Paragraph("Available Skills:"),
			prompt.SkillsList(skillInfos()),
		))

	return p
}

func skillInfos() []prompt.SkillInfo {
	all := skill.List()
	infos := make([]prompt.SkillInfo, 0, len(all))
	for _, s := range all {
		infos = append(infos, prompt.SkillInfo{
			Name:         s.Name,
			Description:  s.Description,
			WhenToUse:    s.WhenToUse,
			ArgumentHint: s.ArgumentHint,
			Arguments:    s.Arguments,
			Context:      s.Context,
		})
	}
	return infos
}
