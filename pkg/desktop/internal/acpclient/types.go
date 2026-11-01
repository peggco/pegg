package acpclient

import "encoding/json"

type ContentBlock struct {
	Type     string            `json:"type"`
	Text     string            `json:"text,omitempty"`
	Data     string            `json:"data,omitempty"`
	MimeType string            `json:"mimeType,omitempty"`
	URI      string            `json:"uri,omitempty"`
	Name     string            `json:"name,omitempty"`
	Resource *EmbeddedResource `json:"resource,omitempty"`
}

type EmbeddedResource struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

type ClientCapabilities struct {
	FS       *FSCapabilities `json:"fs,omitempty"`
	Terminal bool            `json:"terminal,omitempty"`
}

type FSCapabilities struct {
	ReadTextFile  bool `json:"readTextFile,omitempty"`
	WriteTextFile bool `json:"writeTextFile,omitempty"`
}

type InitializeParams struct {
	ProtocolVersion    int                `json:"protocolVersion"`
	ClientCapabilities ClientCapabilities `json:"clientCapabilities,omitempty"`
	ClientInfo         ClientInfo         `json:"clientInfo,omitempty"`
}

type PeggCapabilities struct {
	Ask         bool `json:"ask,omitempty"`
	Permissions bool `json:"permissions,omitempty"`
	Models      bool `json:"models,omitempty"`
	Commands    bool `json:"commands,omitempty"`
	Attachments bool `json:"attachments,omitempty"`
	Diffs       bool `json:"diffs,omitempty"`
	Plan        bool `json:"plan,omitempty"`
	History     bool `json:"history,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion   int `json:"protocolVersion"`
	AgentCapabilities struct {
		LoadSession        bool `json:"loadSession,omitempty"`
		PromptCapabilities struct {
			Image           bool `json:"image,omitempty"`
			Audio           bool `json:"audio,omitempty"`
			EmbeddedContext bool `json:"embeddedContext,omitempty"`
		} `json:"promptCapabilities,omitempty"`
		Pegg *PeggCapabilities `json:"pegg,omitempty"`
	} `json:"agentCapabilities"`
	AgentInfo *ClientInfo `json:"agentInfo,omitempty"`
}

type SessionNewParams struct {
	Cwd                   string   `json:"cwd"`
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
}

type SessionNewResult struct {
	SessionID      string         `json:"sessionId"`
	PeggSessionID  string         `json:"peggSessionId,omitempty"`
	Mode           string         `json:"mode,omitempty"`
	Provider       string         `json:"provider,omitempty"`
	Model          string         `json:"model,omitempty"`
	PermissionMode string         `json:"permissionMode,omitempty"`
	Commands       []SlashCommand `json:"commands,omitempty"`
}

type SlashCommand struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type SessionLoadParams struct {
	SessionID     string `json:"sessionId"`
	Cwd           string `json:"cwd"`
	HistoryLimit  int    `json:"historyLimit,omitempty"`
	HistoryBefore int    `json:"historyBefore,omitempty"`
}

type SessionHistoryParams struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	BeforeSeq int    `json:"beforeSeq,omitempty"`
}

type SessionHistoryResult struct {
	OldestSeq int  `json:"oldestSeq"`
	HasMore   bool `json:"hasMore"`
}

type SessionResumeParams struct {
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
}

type SessionPromptParams struct {
	SessionID string         `json:"sessionId"`
	Prompt    []ContentBlock `json:"prompt"`
}

type SessionPromptResult struct {
	StopReason string `json:"stopReason"`
	Usage      *Usage `json:"usage,omitempty"`
}

type Usage struct {
	Used  int   `json:"used"`
	Size  int   `json:"size"`
	Cost  *Cost `json:"cost,omitempty"`
	Total int   `json:"total_tokens,omitempty"`
}

type Cost struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type SessionListParams struct {
	Cursor string `json:"cursor,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Cwd    string `json:"cwd,omitempty"`
	All    bool   `json:"all,omitempty"`
}

type SessionListItem struct {
	ID         string       `json:"id"`
	Title      string       `json:"title"`
	ProjectDir string       `json:"projectDir,omitempty"`
	Provider   string       `json:"provider,omitempty"`
	Model      string       `json:"model,omitempty"`
	Mode       string       `json:"mode,omitempty"`
	Usage      SessionUsage `json:"usage,omitempty"`
	CreatedAt  string       `json:"createdAt"`
	UpdatedAt  string       `json:"updatedAt"`
}

type SessionUsage struct {
	PromptTokens     int     `json:"prompt_tokens,omitempty"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	TotalTokens      int     `json:"total_tokens,omitempty"`
	Cost             float64 `json:"cost,omitempty"`
}

type SessionListResult struct {
	Sessions   []SessionListItem `json:"sessions"`
	NextCursor string            `json:"nextCursor,omitempty"`
}

type SessionRenameParams struct {
	SessionID string `json:"sessionId"`
	Title     string `json:"title"`
}

type SessionIDParams struct {
	SessionID string `json:"sessionId"`
}

type SessionForkParams struct {
	SessionID        string `json:"sessionId"`
	MessageID        string `json:"messageId,omitempty"`
	UserMessageIndex *int   `json:"userMessageIndex,omitempty"`
}

type SessionForkResult struct {
	SessionID string `json:"sessionId"`
	Title     string `json:"title,omitempty"`
	ParentID  string `json:"parentId,omitempty"`
}

type SessionRevertResult struct {
	Removed    int    `json:"removed"`
	SnapshotID string `json:"snapshotId,omitempty"`
}

type SessionSetModeParams struct {
	SessionID string `json:"sessionId"`
	ModeID    string `json:"modeId"`
}

type SessionSetModelParams struct {
	SessionID       string `json:"sessionId"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
}

type ModelEntry struct {
	Provider string    `json:"provider"`
	Model    ModelInfo `json:"model"`
}

type ModelInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
}

type ModelListResult struct {
	Models []ModelEntry `json:"models"`
}

type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

type PermissionToolCall struct {
	ToolCallID string `json:"toolCallId,omitempty"`
	Title      string `json:"title,omitempty"`
	Kind       string `json:"kind,omitempty"`
	RawInput   any    `json:"rawInput,omitempty"`
}

type PermissionParams struct {
	SessionID string             `json:"sessionId"`
	Question  string             `json:"question,omitempty"`
	ToolCall  PermissionToolCall `json:"toolCall"`
	Options   []PermissionOption `json:"options"`
}

type AskQuestion struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Type     string   `json:"type"`
	Options  []string `json:"options,omitempty"`
	Required bool     `json:"required,omitempty"`
}

type AskParams struct {
	SessionID string        `json:"sessionId"`
	AgentID   string        `json:"agentId,omitempty"`
	AgentName string        `json:"agentName,omitempty"`
	Questions []AskQuestion `json:"questions"`
}

type STTResult struct {
	Text     string `json:"text"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	if e == nil {
		return "acp error"
	}
	return e.Message
}
