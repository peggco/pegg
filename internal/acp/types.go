package acp

type SessionId string

type ContentBlock struct {
	Type        string            `json:"type"`
	Text        string            `json:"text,omitempty"`
	Data        string            `json:"data,omitempty"`
	MimeType    string            `json:"mimeType,omitempty"`
	URI         string            `json:"uri,omitempty"`
	Resource    *EmbeddedResource `json:"resource,omitempty"`
	Annotations *Annotations      `json:"annotations,omitempty"`
}

type EmbeddedResource struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

type Annotations struct {
	Audience     []string `json:"audience,omitempty"`
	Priority     float64  `json:"priority,omitempty"`
	LastModified string   `json:"lastModified,omitempty"`
}

type StopReason string

const (
	StopReasonEndTurn     StopReason = "end_turn"
	StopReasonMaxTokens   StopReason = "max_tokens"
	StopReasonMaxRequests StopReason = "max_turn_requests"
	StopReasonRefusal     StopReason = "refusal"
	StopReasonCancelled   StopReason = "cancelled"
	StopReasonError       StopReason = "error"
)

type SessionUpdate struct {
	SessionUpdate string             `json:"sessionUpdate"`
	MessageID     string             `json:"messageId,omitempty"`
	Content       any                `json:"content,omitempty"`
	ToolCallID    string             `json:"toolCallId,omitempty"`
	ToolName      string             `json:"toolName,omitempty"`
	Title         string             `json:"title,omitempty"`
	Kind          string             `json:"kind,omitempty"`
	Status        string             `json:"status,omitempty"`
	Locations     []ToolCallLocation `json:"locations,omitempty"`
	RawInput      any                `json:"rawInput,omitempty"`
	RawOutput     any                `json:"rawOutput,omitempty"`
	Entries       []PlanEntry        `json:"entries,omitempty"`
	Used          int                `json:"used,omitempty"`
	Size          int                `json:"size,omitempty"`
	Cost          *Cost              `json:"cost,omitempty"`
	Mode          string             `json:"mode,omitempty"`
	Provider      string             `json:"provider,omitempty"`
	Model         string             `json:"model,omitempty"`
	Commands      []SlashCommand     `json:"commands,omitempty"`
	AgentID       string             `json:"agentId,omitempty"`
	AgentName     string             `json:"agentName,omitempty"`
	Error         string             `json:"error,omitempty"`
	Contents      []ToolCallContent  `json:"contents,omitempty"`
	RunID         string             `json:"runId,omitempty"`
	RunState      string             `json:"runState,omitempty"`
	BaseSeq       int                `json:"baseSeq,omitempty"`
	Seq           int                `json:"seq,omitempty"`
	HistoryPage   bool               `json:"historyPage,omitempty"`
}

type ToolCallContent struct {
	Type       string        `json:"type"`
	Content    *ContentBlock `json:"content,omitempty"`
	Diff       *Diff         `json:"diff,omitempty"`
	TerminalId string        `json:"terminalId,omitempty"`
}

type Diff struct {
	Path    string `json:"path"`
	OldText string `json:"oldText,omitempty"`
	NewText string `json:"newText"`
}

type ToolCallLocation struct {
	Path string `json:"path"`
	Line int    `json:"line,omitempty"`
}

type PlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority,omitempty"`
	Status   string `json:"status,omitempty"`
}

type Cost struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type SlashCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ToolKind string

const (
	ToolKindRead    ToolKind = "read"
	ToolKindEdit    ToolKind = "edit"
	ToolKindDelete  ToolKind = "delete"
	ToolKindMove    ToolKind = "move"
	ToolKindSearch  ToolKind = "search"
	ToolKindExecute ToolKind = "execute"
	ToolKindThink   ToolKind = "think"
	ToolKindFetch   ToolKind = "fetch"
	ToolKindSwitch  ToolKind = "switch_mode"
	ToolKindOther   ToolKind = "other"
)

type ToolCallStatus string

const (
	ToolCallPending    ToolCallStatus = "pending"
	ToolCallInProgress ToolCallStatus = "in_progress"
	ToolCallCompleted  ToolCallStatus = "completed"
	ToolCallFailed     ToolCallStatus = "failed"
	ToolCallCancelled  ToolCallStatus = "cancelled"
)

type PermissionOption struct {
	OptionId string               `json:"optionId"`
	Name     string               `json:"name"`
	Kind     PermissionOptionKind `json:"kind"`
}

type PermissionOptionKind string

const (
	PermAllowOnce    PermissionOptionKind = "allow_once"
	PermAllowAlways  PermissionOptionKind = "allow_always"
	PermRejectOnce   PermissionOptionKind = "reject_once"
	PermRejectAlways PermissionOptionKind = "reject_always"
)
