package acpclient

import "encoding/json"

type Meta map[string]any

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
}

type EmbeddedResource struct {
	Meta     Meta   `json:"_meta,omitempty"`
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

type ClientInfo struct {
	Meta    Meta   `json:"_meta,omitempty"`
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version,omitempty"`
}

type ClientCapabilities struct {
	Meta        Meta                     `json:"_meta,omitempty"`
	FS          *FSCapabilities          `json:"fs,omitempty"`
	Terminal    bool                     `json:"terminal,omitempty"`
	Elicitation *ElicitationCapabilities `json:"elicitation,omitempty"`
	Auth        *ClientAuthCapabilities  `json:"auth,omitempty"`
	Session     *ClientSessionCaps       `json:"session,omitempty"`
}

type FSCapabilities struct {
	Meta          Meta `json:"_meta,omitempty"`
	ReadTextFile  bool `json:"readTextFile,omitempty"`
	WriteTextFile bool `json:"writeTextFile,omitempty"`
}

type ElicitationCapabilities struct {
	Meta Meta      `json:"_meta,omitempty"`
	Form *struct{} `json:"form,omitempty"`
	URL  *struct{} `json:"url,omitempty"`
}

type ClientAuthCapabilities struct {
	Meta     Meta `json:"_meta,omitempty"`
	Terminal bool `json:"terminal,omitempty"`
}

type ClientSessionCaps struct {
	Meta          Meta                 `json:"_meta,omitempty"`
	ConfigOptions *ClientConfigOptions `json:"configOptions,omitempty"`
	Compaction    *struct{}            `json:"compaction,omitempty"`
	Notices       *struct{}            `json:"notices,omitempty"`
}

type ClientConfigOptions struct {
	Meta    Meta      `json:"_meta,omitempty"`
	Boolean *struct{} `json:"boolean,omitempty"`
}

type InitializeParams struct {
	Meta               Meta               `json:"_meta,omitempty"`
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
	Terminals   bool `json:"terminals,omitempty"`
	Elicitation bool `json:"elicitation,omitempty"`
}

type AgentCapabilities struct {
	Meta               Meta                `json:"_meta,omitempty"`
	LoadSession        bool                `json:"loadSession,omitempty"`
	PromptCapabilities *PromptCapabilities `json:"promptCapabilities,omitempty"`
	SessionCaps        *AgentSessionCaps   `json:"sessionCapabilities,omitempty"`
	Auth               *AgentAuthCaps      `json:"auth,omitempty"`
}

type PromptCapabilities struct {
	Meta            Meta `json:"_meta,omitempty"`
	Image           bool `json:"image,omitempty"`
	Audio           bool `json:"audio,omitempty"`
	EmbeddedContext bool `json:"embeddedContext,omitempty"`
}

type AgentSessionCaps struct {
	Meta                  Meta      `json:"_meta,omitempty"`
	Delete                *struct{} `json:"delete,omitempty"`
	Resume                *struct{} `json:"resume,omitempty"`
	Close                 *struct{} `json:"close,omitempty"`
	List                  *struct{} `json:"list,omitempty"`
	AdditionalDirectories *struct{} `json:"additionalDirectories,omitempty"`
}

type AgentAuthCaps struct {
	Meta   Meta      `json:"_meta,omitempty"`
	Logout *struct{} `json:"logout,omitempty"`
}

type InitializeResult struct {
	Meta              Meta              `json:"_meta,omitempty"`
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
	AgentInfo         *ClientInfo       `json:"agentInfo,omitempty"`
	AuthMethods       []AuthMethod      `json:"authMethods,omitempty"`
}

type AuthMethod struct {
	Meta        Meta              `json:"_meta,omitempty"`
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        string            `json:"type,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

func (r *InitializeResult) Pegg() *PeggCapabilities {
	if r == nil {
		return nil
	}
	raw, ok := r.AgentCapabilities.Meta["com.pegg/capabilities"]
	if !ok || raw == nil {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var caps PeggCapabilities
	if err := json.Unmarshal(data, &caps); err != nil {
		return nil
	}
	return &caps
}

type SessionNewParams struct {
	Meta                  Meta     `json:"_meta,omitempty"`
	Cwd                   string   `json:"cwd"`
	MCPServers            []any    `json:"mcpServers,omitempty"`
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
}

type SessionMode struct {
	Meta        Meta   `json:"_meta,omitempty"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type SessionModeState struct {
	Meta           Meta          `json:"_meta,omitempty"`
	CurrentModeID  string        `json:"currentModeId"`
	AvailableModes []SessionMode `json:"availableModes"`
}

type SessionConfigSelectOption struct {
	Meta        Meta   `json:"_meta,omitempty"`
	Value       string `json:"value"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type SessionConfigOption struct {
	Meta         Meta   `json:"_meta,omitempty"`
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Category     string `json:"category,omitempty"`
	Type         string `json:"type"`
	CurrentValue any    `json:"currentValue"`
	Options      any    `json:"options,omitempty"`
}

type SessionNewResult struct {
	Meta          Meta                  `json:"_meta,omitempty"`
	SessionID     string                `json:"sessionId"`
	Modes         *SessionModeState     `json:"modes,omitempty"`
	ConfigOptions []SessionConfigOption `json:"configOptions,omitempty"`
}

type SessionStateResult struct {
	Meta          Meta                  `json:"_meta,omitempty"`
	Modes         *SessionModeState     `json:"modes,omitempty"`
	ConfigOptions []SessionConfigOption `json:"configOptions,omitempty"`
}

type SlashCommand struct {
	Meta        Meta   `json:"_meta,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Input       *struct {
		Hint string `json:"hint"`
	} `json:"input,omitempty"`
}

type SkillsReloadResult struct {
	Commands []SlashCommand `json:"commands"`
}

type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
	Path        string `json:"path,omitempty"`
	WhenToUse   string `json:"whenToUse,omitempty"`
}

type SkillsListResult struct {
	Skills []SkillInfo `json:"skills"`
}

type SessionLoadParams struct {
	Meta                  Meta     `json:"_meta,omitempty"`
	SessionID             string   `json:"sessionId"`
	Cwd                   string   `json:"cwd"`
	MCPServers            []any    `json:"mcpServers,omitempty"`
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
	HistoryLimit          int      `json:"historyLimit,omitempty"`
	HistoryBefore         int      `json:"historyBefore,omitempty"`
}

type SessionHistoryParams struct {
	Meta      Meta   `json:"_meta,omitempty"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	BeforeSeq int    `json:"beforeSeq,omitempty"`
}

type SessionHistoryResult struct {
	Meta      Meta `json:"_meta,omitempty"`
	OldestSeq int  `json:"oldestSeq"`
	HasMore   bool `json:"hasMore"`
}

type SessionResumeParams struct {
	Meta                  Meta     `json:"_meta,omitempty"`
	SessionID             string   `json:"sessionId"`
	Cwd                   string   `json:"cwd"`
	MCPServers            []any    `json:"mcpServers,omitempty"`
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
}

type SessionPromptParams struct {
	Meta      Meta           `json:"_meta,omitempty"`
	SessionID string         `json:"sessionId"`
	Prompt    []ContentBlock `json:"prompt"`
}

type SessionPromptResult struct {
	Meta       Meta   `json:"_meta,omitempty"`
	StopReason string `json:"stopReason"`
}

type SessionListParams struct {
	Meta   Meta   `json:"_meta,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	Cwd    string `json:"cwd,omitempty"`
}

type SessionInfo struct {
	Meta                  Meta     `json:"_meta,omitempty"`
	SessionID             string   `json:"sessionId"`
	Cwd                   string   `json:"cwd"`
	Title                 string   `json:"title,omitempty"`
	UpdatedAt             string   `json:"updatedAt,omitempty"`
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
}

type SessionListResult struct {
	Meta       Meta          `json:"_meta,omitempty"`
	Sessions   []SessionInfo `json:"sessions"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

type SessionRenameParams struct {
	Meta      Meta   `json:"_meta,omitempty"`
	SessionID string `json:"sessionId"`
	Title     string `json:"title"`
}

type SessionIDParams struct {
	Meta      Meta   `json:"_meta,omitempty"`
	SessionID string `json:"sessionId"`
}

type SessionForkParams struct {
	Meta             Meta   `json:"_meta,omitempty"`
	SessionID        string `json:"sessionId"`
	MessageID        string `json:"messageId,omitempty"`
	UserMessageIndex *int   `json:"userMessageIndex,omitempty"`
}

type SessionForkResult struct {
	Meta      Meta   `json:"_meta,omitempty"`
	SessionID string `json:"sessionId"`
	Title     string `json:"title,omitempty"`
	ParentID  string `json:"parentId,omitempty"`
}

type SessionRevertResult struct {
	Meta       Meta   `json:"_meta,omitempty"`
	Removed    int    `json:"removed"`
	SnapshotID string `json:"snapshotId,omitempty"`
}

type SessionSetModeParams struct {
	Meta      Meta   `json:"_meta,omitempty"`
	SessionID string `json:"sessionId"`
	ModeID    string `json:"modeId"`
}

type SessionSetModelParams struct {
	Meta            Meta   `json:"_meta,omitempty"`
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
	Meta     Meta   `json:"_meta,omitempty"`
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
}

type PermissionToolCall struct {
	Meta       Meta   `json:"_meta,omitempty"`
	ToolCallID string `json:"toolCallId,omitempty"`
	Title      string `json:"title,omitempty"`
	Kind       string `json:"kind,omitempty"`
	RawInput   any    `json:"rawInput,omitempty"`
}

type PermissionParams struct {
	Meta      Meta               `json:"_meta,omitempty"`
	SessionID string             `json:"sessionId"`
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
	Meta      Meta          `json:"_meta,omitempty"`
	SessionID string        `json:"sessionId"`
	AgentID   string        `json:"agentId,omitempty"`
	AgentName string        `json:"agentName,omitempty"`
	Questions []AskQuestion `json:"questions"`
}

type STTResult struct {
	Meta     Meta   `json:"_meta,omitempty"`
	Text     string `json:"text"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

type EnvVar struct {
	Meta  Meta   `json:"_meta,omitempty"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type TerminalExitStatus struct {
	Meta     Meta    `json:"_meta,omitempty"`
	ExitCode *int    `json:"exitCode,omitempty"`
	Signal   *string `json:"signal,omitempty"`
}

type TerminalCreateParams struct {
	Meta            Meta     `json:"_meta,omitempty"`
	SessionID       string   `json:"sessionId"`
	Command         string   `json:"command"`
	Args            []string `json:"args,omitempty"`
	Env             []EnvVar `json:"env,omitempty"`
	Cwd             string   `json:"cwd,omitempty"`
	OutputByteLimit *int     `json:"outputByteLimit,omitempty"`
}

type TerminalIDParams struct {
	Meta       Meta   `json:"_meta,omitempty"`
	SessionID  string `json:"sessionId"`
	TerminalID string `json:"terminalId"`
}

type TerminalOutputResult struct {
	Meta       Meta                `json:"_meta,omitempty"`
	Output     string              `json:"output"`
	Truncated  bool                `json:"truncated"`
	ExitStatus *TerminalExitStatus `json:"exitStatus,omitempty"`
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
	SessionID       string             `json:"sessionId,omitempty"`
	ToolCallID      string             `json:"toolCallId,omitempty"`
	RequestID       any                `json:"requestId,omitempty"`
	RequestedSchema *ElicitationSchema `json:"requestedSchema,omitempty"`
	ElicitationID   string             `json:"elicitationId,omitempty"`
	URL             string             `json:"url,omitempty"`
}

type ElicitationCreateResult struct {
	Meta    Meta   `json:"_meta,omitempty"`
	Action  string `json:"action"`
	Content any    `json:"content,omitempty"`
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
