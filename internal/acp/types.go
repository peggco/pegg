package acp

type Meta map[string]any

type (
	SessionId            string
	MessageId            string
	ToolCallId           string
	TerminalId           string
	PermissionOptionId   string
	SessionModeId        string
	SessionConfigId      string
	SessionConfigValueId string
	SessionConfigGroupId string
	AuthMethodId         string
	ElicitationId        string
)

type Annotations struct {
	Meta         Meta     `json:"_meta,omitempty"`
	Audience     []string `json:"audience,omitempty"`
	Priority     *float64 `json:"priority,omitempty"`
	LastModified *string  `json:"lastModified,omitempty"`
}

type ContentBlock struct {
	Meta        Meta              `json:"_meta,omitempty"`
	Type        string            `json:"type"`
	Text        string            `json:"text,omitempty"`
	Data        string            `json:"data,omitempty"`
	MimeType    string            `json:"mimeType,omitempty"`
	URI         string            `json:"uri,omitempty"`
	Name        string            `json:"name,omitempty"`
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Size        *int64            `json:"size,omitempty"`
	Resource    *EmbeddedResource `json:"resource,omitempty"`
	Annotations *Annotations      `json:"annotations,omitempty"`
}

type EmbeddedResource struct {
	Meta        Meta         `json:"_meta,omitempty"`
	URI         string       `json:"uri"`
	MimeType    string       `json:"mimeType,omitempty"`
	Text        string       `json:"text,omitempty"`
	Blob        string       `json:"blob,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
}

type AttachmentInfo struct {
	Name      string `json:"name,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
	Type      string `json:"type,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Path      string `json:"path,omitempty"`
}

type StopReason string

const (
	StopReasonEndTurn        StopReason = "end_turn"
	StopReasonMaxTokens      StopReason = "max_tokens"
	StopReasonMaxTurnRequest StopReason = "max_turn_requests"
	StopReasonRefusal        StopReason = "refusal"
	StopReasonCancelled      StopReason = "cancelled"
)

type ToolKind string

const (
	ToolKindRead       ToolKind = "read"
	ToolKindEdit       ToolKind = "edit"
	ToolKindDelete     ToolKind = "delete"
	ToolKindMove       ToolKind = "move"
	ToolKindSearch     ToolKind = "search"
	ToolKindExecute    ToolKind = "execute"
	ToolKindThink      ToolKind = "think"
	ToolKindFetch      ToolKind = "fetch"
	ToolKindSwitchMode ToolKind = "switch_mode"
	ToolKindOther      ToolKind = "other"
)

type ToolCallStatus string

const (
	ToolCallPending    ToolCallStatus = "pending"
	ToolCallInProgress ToolCallStatus = "in_progress"
	ToolCallCompleted  ToolCallStatus = "completed"
	ToolCallFailed     ToolCallStatus = "failed"
)

type PermissionOptionKind string

const (
	PermAllowOnce    PermissionOptionKind = "allow_once"
	PermAllowAlways  PermissionOptionKind = "allow_always"
	PermRejectOnce   PermissionOptionKind = "reject_once"
	PermRejectAlways PermissionOptionKind = "reject_always"
)

type PermissionOption struct {
	Meta     Meta                 `json:"_meta,omitempty"`
	OptionId PermissionOptionId   `json:"optionId"`
	Name     string               `json:"name"`
	Kind     PermissionOptionKind `json:"kind"`
}

type ToolCallContent struct {
	Meta       Meta          `json:"_meta,omitempty"`
	Type       string        `json:"type"`
	Content    *ContentBlock `json:"content,omitempty"`
	Diff       *Diff         `json:"diff,omitempty"`
	TerminalId TerminalId    `json:"terminalId,omitempty"`
}

type Diff struct {
	Meta    Meta    `json:"_meta,omitempty"`
	Path    string  `json:"path"`
	OldText *string `json:"oldText,omitempty"`
	NewText string  `json:"newText"`
}

type ToolCallLocation struct {
	Meta Meta   `json:"_meta,omitempty"`
	Path string `json:"path"`
	Line *int   `json:"line,omitempty"`
}

type PlanEntry struct {
	Meta     Meta   `json:"_meta,omitempty"`
	Content  string `json:"content"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

type Cost struct {
	Meta     Meta    `json:"_meta,omitempty"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type AvailableCommandInput struct {
	Meta Meta   `json:"_meta,omitempty"`
	Hint string `json:"hint"`
}

type AvailableCommand struct {
	Meta        Meta                   `json:"_meta,omitempty"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       *AvailableCommandInput `json:"input,omitempty"`
}

type SessionMode struct {
	Meta        Meta   `json:"_meta,omitempty"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type SessionModeState struct {
	Meta           Meta          `json:"_meta,omitempty"`
	CurrentModeId  SessionModeId `json:"currentModeId"`
	AvailableModes []SessionMode `json:"availableModes"`
}

type SessionConfigOptionCategory string

const (
	ConfigCategoryMode         SessionConfigOptionCategory = "mode"
	ConfigCategoryModel        SessionConfigOptionCategory = "model"
	ConfigCategoryModelConfig  SessionConfigOptionCategory = "model_config"
	ConfigCategoryThoughtLevel SessionConfigOptionCategory = "thought_level"
)

type SessionConfigSelectOption struct {
	Meta        Meta                 `json:"_meta,omitempty"`
	Value       SessionConfigValueId `json:"value"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
}

type SessionConfigSelectGroup struct {
	Meta    Meta                        `json:"_meta,omitempty"`
	Group   SessionConfigGroupId        `json:"group"`
	Name    string                      `json:"name"`
	Options []SessionConfigSelectOption `json:"options"`
}

type SessionConfigOption struct {
	Meta         Meta                        `json:"_meta,omitempty"`
	ID           SessionConfigId             `json:"id"`
	Name         string                      `json:"name"`
	Description  string                      `json:"description,omitempty"`
	Category     SessionConfigOptionCategory `json:"category,omitempty"`
	Type         string                      `json:"type"`
	CurrentValue any                         `json:"currentValue"`
	Options      any                         `json:"options,omitempty"`
}

type ModelInfo struct {
	Meta        Meta   `json:"_meta,omitempty"`
	ModelId     string `json:"modelId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type SessionModelState struct {
	Meta            Meta        `json:"_meta,omitempty"`
	CurrentModelId  string      `json:"currentModelId"`
	AvailableModels []ModelInfo `json:"availableModels"`
}

type NoticeSeverity string

const (
	NoticeInfo    NoticeSeverity = "info"
	NoticeWarning NoticeSeverity = "warning"
	NoticeError   NoticeSeverity = "error"
)

type CompactionStatus string

const (
	CompactionInProgress CompactionStatus = "in_progress"
	CompactionCompleted  CompactionStatus = "completed"
	CompactionFailed     CompactionStatus = "failed"
	CompactionCancelled  CompactionStatus = "cancelled"
)

type SessionUpdate struct {
	Meta Meta `json:"_meta,omitempty"`

	SessionUpdate string `json:"sessionUpdate"`

	MessageID MessageId `json:"messageId,omitempty"`
	Content   any       `json:"content,omitempty"`

	ToolCallID ToolCallId         `json:"toolCallId,omitempty"`
	Title      string             `json:"title,omitempty"`
	Name       string             `json:"name,omitempty"`
	Kind       ToolKind           `json:"kind,omitempty"`
	Status     string             `json:"status,omitempty"`
	Locations  []ToolCallLocation `json:"locations,omitempty"`
	RawInput   any                `json:"rawInput,omitempty"`
	RawOutput  any                `json:"rawOutput,omitempty"`

	Entries []PlanEntry `json:"entries,omitempty"`

	AvailableCommands []AvailableCommand `json:"availableCommands,omitempty"`

	CurrentModeId SessionModeId `json:"currentModeId,omitempty"`

	ConfigOptions []SessionConfigOption `json:"configOptions,omitempty"`

	UpdatedAt *string `json:"updatedAt,omitempty"`

	Used uint64 `json:"used,omitempty"`
	Size uint64 `json:"size,omitempty"`
	Cost *Cost  `json:"cost,omitempty"`

	Severity    NoticeSeverity `json:"severity,omitempty"`
	Description string         `json:"description,omitempty"`

	CompactionId CompactionId   `json:"compactionId,omitempty"`
	Summary      []ContentBlock `json:"summary,omitempty"`
	Error        string         `json:"error,omitempty"`
}

type CompactionId string

type ToolCallUpdate struct {
	Meta       Meta               `json:"_meta,omitempty"`
	ToolCallID ToolCallId         `json:"toolCallId"`
	Title      *string            `json:"title,omitempty"`
	Name       *string            `json:"name,omitempty"`
	Kind       *ToolKind          `json:"kind,omitempty"`
	Status     *ToolCallStatus    `json:"status,omitempty"`
	Content    []ToolCallContent  `json:"content,omitempty"`
	Locations  []ToolCallLocation `json:"locations,omitempty"`
	RawInput   any                `json:"rawInput,omitempty"`
	RawOutput  any                `json:"rawOutput,omitempty"`
}

type TerminalExitStatus struct {
	Meta     Meta    `json:"_meta,omitempty"`
	ExitCode *int    `json:"exitCode,omitempty"`
	Signal   *string `json:"signal,omitempty"`
}

type Implementation struct {
	Meta    Meta   `json:"_meta,omitempty"`
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

type SystemPromptMode string

const (
	SystemPromptAppend   SystemPromptMode = "append"
	SystemPromptOverride SystemPromptMode = "override"
)

type SystemPrompt struct {
	Meta    Meta             `json:"_meta,omitempty"`
	Mode    SystemPromptMode `json:"mode"`
	Content []ContentBlock   `json:"content"`
}

type AuthMethod struct {
	Meta        Meta              `json:"_meta,omitempty"`
	ID          AuthMethodId      `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        string            `json:"type,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

type ElicitationSchema struct {
	Meta        Meta           `json:"_meta,omitempty"`
	Type        string         `json:"type"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Properties  map[string]any `json:"properties,omitempty"`
	Required    []string       `json:"required,omitempty"`
}

type ElicitationCreateParams struct {
	Meta            Meta               `json:"_meta,omitempty"`
	Mode            string             `json:"mode"`
	Message         string             `json:"message"`
	SessionID       SessionId          `json:"sessionId,omitempty"`
	ToolCallID      ToolCallId         `json:"toolCallId,omitempty"`
	RequestID       any                `json:"requestId,omitempty"`
	RequestedSchema *ElicitationSchema `json:"requestedSchema,omitempty"`
	ElicitationID   ElicitationId      `json:"elicitationId,omitempty"`
	URL             string             `json:"url,omitempty"`
}

type ElicitationCreateResult struct {
	Meta    Meta   `json:"_meta,omitempty"`
	Action  string `json:"action"`
	Content any    `json:"content,omitempty"`
}
