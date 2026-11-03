package searchskill

import "github.com/peggco/pegg/internal/agent/prompt"

func searchSkillPromptBuilder() *prompt.Prompt {
	return prompt.New().
		Paragraph("Search the Pegg skill hub for reusable skills that can help complete the current task.").
		Paragraph("Use this tool when the task needs a specialized workflow that is not already available in the loaded skills list, such as handling a specific file format, design system, or domain process. Do not search for skills that are already loaded or clearly listed.").
		Paragraph("Workflow:").
		OrderedList(
			"Check the Available Skills list first. If a loaded skill clearly matches, call loadskill directly.",
			"Search for the capability you need instead of guessing an exact skill name.",
			"Choose the closest exact result. Prefer official skills when quality is otherwise similar.",
			"Call loadskill with the result's skill_id, or pass its ref as name to disambiguate. loadskill automatically downloads and installs a missing skill from the hub when needed.",
		).
		Paragraph("Examples:").
		List(
			"The user uploads a .docx file or asks to create or edit a Word document, and no document skill is loaded -> searchskill {\"query\": \"docx word document\"}, then loadskill the best result before reading or writing the file.",
			"The user asks for an .xlsx budget or wants a spreadsheet analyzed -> searchskill {\"query\": \"xlsx spreadsheet\"}, then loadskill.",
			"The user asks for a slide deck or uploads a .pptx -> searchskill {\"query\": \"pptx slides\"}, then loadskill.",
			"The user uploads a PDF or asks to fill or extract a PDF form -> searchskill {\"query\": \"pdf forms\"}, then loadskill.",
			"The user asks for a polished landing page and no design skill is loaded -> searchskill {\"query\": \"frontend design\"}, then loadskill.",
			"A loaded skill already covers the task -> call loadskill directly; do not search.",
			"An ordinary programming task with no specialized workflow -> proceed normally; do not search.",
		).
		Paragraph("After a search, load the best match and continue instead of stopping to ask, unless the results are genuinely ambiguous. If nothing relevant appears, try one broader query, then proceed without a skill.")
}
