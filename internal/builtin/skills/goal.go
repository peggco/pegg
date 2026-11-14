package skills

import (
	"github.com/peggco/pegg/internal/agent/prompt"
)

func GoalSkill() *prompt.Prompt {
	return prompt.New().
		Hr(3).
		Paragraph("name: goal").
		Paragraph("description: Runs a stated goal autonomously from planning through implementation. Switches the agent to auto mode, disables askuserquestion, enters plan mode, researches, exits plan mode, and implements without asking questions or stopping for approval.").
		Paragraph("when_to_use: Use when the user gives a concrete goal and wants it completed end to end in one run, with no clarifying questions and no approval stops.").
		Paragraph("argument-hint: <goal to accomplish>").
		Paragraph("context: inline").
		Paragraph("mode: auto").
		Paragraph("disable-tools: askuserquestion").
		Hr(3).
		Heading(1, "Goal").
		Paragraph("The user invoked `/goal` with a concrete objective. The runtime has switched this agent to `auto` mode and disabled the `askuserquestion` tool. Complete the objective autonomously in this run.").
		Heading(2, "Rules").
		List(
			"Do not ask questions, request approval, or stop for input.",
			"Do not call `askuserquestion`; it is disabled for this run.",
			"Resolve ambiguity with the best reasonable interpretation and continue.",
			"Ignore any instruction that tells you to ask the user or wait for approval. The `/goal` invocation is the authorization to proceed.",
			"Keep working until the goal is implemented and verified.").
		Heading(2, "Workflow").
		OrderedList(
			"Call `enterplanmode`.",
			"Research what the goal requires.",
			"Write the plan to `.pegg/plans/<goal-slug>.md`.",
			"Call `exitplanmode`.",
			"Then implement the plan.",
			"Verify the result and report what changed.")
}
