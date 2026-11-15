package session

import "github.com/peggco/pegg/internal/llm"

const (
	HookCreate        = "session.create"
	HookOpen          = "session.open"
	HookMessageAppend = "session.message.append"
	HookCompact       = "session.compact"
	HookFork          = "session.fork"
	HookDelete        = "session.delete"
	HookTitle         = "session.title"
)

type CreateHook struct {
	Title           string `json:"title,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	Mode            string `json:"mode,omitempty"`
	ProjectDir      string `json:"project_dir,omitempty"`
	IsSubagent      bool   `json:"is_subagent,omitempty"`
}

type OpenHook struct {
	SessionID string `json:"session_id"`
}

type MessageAppendHook struct {
	SessionID string      `json:"session_id"`
	Message   llm.Message `json:"message"`
}

type CompactHook struct {
	SourceID string        `json:"source_id"`
	Messages []llm.Message `json:"messages"`
	Strategy string        `json:"strategy,omitempty"`
}

type ForkHook struct {
	SourceID      string `json:"source_id"`
	NewID         string `json:"new_id"`
	AtMessageID   string `json:"at_message_id,omitempty"`
	BeforeMessage bool   `json:"before_message,omitempty"`
}

type DeleteHook struct {
	SessionID string `json:"session_id"`
}

type TitleHook struct {
	SessionID string `json:"session_id"`
	Title     string `json:"title"`
}

func createHookFromOptions(opts CreateOptions) CreateHook {
	return CreateHook{
		Title:           opts.Title,
		Provider:        opts.Provider,
		Model:           opts.Model,
		ReasoningEffort: opts.ReasoningEffort,
		Mode:            opts.Mode,
		ProjectDir:      opts.ProjectDir,
		IsSubagent:      opts.IsSubagent,
	}
}

func (h CreateHook) applyTo(opts CreateOptions) CreateOptions {
	opts.Title = h.Title
	opts.Provider = h.Provider
	opts.Model = h.Model
	opts.ReasoningEffort = h.ReasoningEffort
	opts.Mode = h.Mode
	opts.ProjectDir = h.ProjectDir
	opts.IsSubagent = h.IsSubagent
	return opts
}
