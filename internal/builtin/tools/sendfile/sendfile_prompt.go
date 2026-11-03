package sendfile

import "github.com/peggco/pegg/internal/agent/prompt"

func sendFileToolPromptBuilder() *prompt.Prompt {
	return prompt.New().
		Paragraph("Sends one or more files to the user so they can open or download them from the chat.").
		Paragraph("Use this tool when the user asks for a file, or when you produced an artifact the user needs to download: archives, exports, reports, generated images, binaries, or logs.").
		Paragraph("Usage:").
		List("The files parameter is a list of paths. Relative paths resolve against the workspace root; absolute paths inside the workspace also work.",
			"Only regular, non-empty files can be sent. Directories are rejected; archive them first with bash if needed.",
			"Files must stay inside the workspace. Sending a file from outside the workspace asks the user for permission.",
			"Each file appears as a download card in the user's client. The file contents are not added to the conversation, so never read a binary file just to send it.",
			"Send the original file, not a copy, and do not paste the file contents into your reply.",
			"At most 20 files per call, 200 MB per file, and 500 MB in total.",
			"Briefly describe what each file is in your reply so the user knows what they are downloading.")
}
