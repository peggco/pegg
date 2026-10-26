package connect

import (
	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent"
	"github.com/peggco/pegg/internal/llm"
)

const ProtocolVersion = 1

const (
	frameHello   = "hello"
	frameWelcome = "welcome"
	frameRequest = "request"
	frameResp    = "response"
	frameEvent   = "event"
	framePing    = "ping"
	framePong    = "pong"
	frameError   = "error"

	frameHeartbeat = "heartbeat"
)

const (
	eventRunStarted         = "run.started"
	eventRunInput           = "run.input"
	eventRunToken           = "run.token"
	eventRunReasoning       = "run.reasoning"
	eventRunToolCall        = "run.tool_call"
	eventRunToolResult      = "run.tool_result"
	eventRunAsk             = "run.ask"
	eventRunAskResolved     = "run.ask.resolved"
	eventRunPermission      = "run.permission"
	eventRunUsage           = "run.usage"
	eventRunCompaction      = "run.compaction"
	eventRunProgress        = "run.progress"
	eventRunDone            = "run.done"
	eventRunError           = "run.error"
	eventRunAgentError      = "run.agent_error"
	eventRunWarning         = "run.warning"
	eventRunWarnClear       = "run.warning.clear"
	eventRunSubagentStarted = "run.subagent.started"
	eventRunSubagentMessage = "run.subagent.message"
	eventRunSubagentUsage   = "run.subagent.usage"
	eventRunSubagentDone    = "run.subagent.done"
	eventRunSubagentError   = "run.subagent.error"
	eventSessionUpdate      = "session.updated"
	eventSettings           = "settings.changed"
	eventStatus             = "status.changed"
	eventSTTStarted         = "stt.started"
	eventSTTPartial         = "stt.partial"
	eventSTTFinished        = "stt.finished"
	eventSTTError           = "stt.error"
	eventHeartbeat          = "heartbeat"
)

const (
	ScopeRead          = "read"
	ScopeSessionsRead  = "sessions:read"
	ScopeSessionsWrite = "sessions:manage"
	ScopeChat          = "chat"
	ScopeSettingsRead  = "settings:read"
	ScopeSettingsWrite = "settings:write"
	ScopeExec          = "exec"
	ScopeAdmin         = "admin"
)

const (
	ErrCodeBadRequest   = "bad_request"
	ErrCodeForbidden    = "forbidden"
	ErrCodeNotFound     = "not_found"
	ErrCodeConflict     = "conflict"
	ErrCodeRateLimited  = "rate_limited"
	ErrCodeUnavailable  = "unavailable"
	ErrCodeInternal     = "internal"
	ErrCodeUnauthorized = "unauthorized"
)

type Frame struct {
	V         int             `json:"v"`
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	Method    string          `json:"method,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	RunID     string          `json:"run_id,omitempty"`
	Event     string          `json:"event,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Error     *FrameError     `json:"error,omitempty"`
	TS        int64           `json:"ts,omitempty"`
}

type FrameError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *FrameError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}

type HelloPayload struct {
	ProtocolVersion int      `json:"protocol_version"`
	DeviceID        string   `json:"device_id"`
	DeviceName      string   `json:"device_name"`
	PublicKey       string   `json:"public_key"`
	PeggVersion     string   `json:"pegg_version"`
	Workspace       string   `json:"workspace"`
	Capabilities    []string `json:"capabilities"`
	Nonce           string   `json:"nonce"`
	TS              int64    `json:"ts"`
	Signature       string   `json:"signature"`
}

type WelcomePayload struct {
	ProtocolVersion int      `json:"protocol_version"`
	SessionID       string   `json:"session_id"`
	ServerTime      int64    `json:"server_time"`
	GrantedScopes   []string `json:"granted_scopes"`
	HeartbeatSecs   int      `json:"heartbeat_secs"`
	MaxFrameBytes   int      `json:"max_frame_bytes"`
}

type ModelInfo struct {
	Provider         string                `json:"provider"`
	ID               string                `json:"id"`
	Name             string                `json:"name,omitempty"`
	ReasoningOptions []llm.ReasoningOption `json:"reasoning_options,omitempty"`
}

type RunInfo struct {
	ID        string              `json:"id"`
	SessionID string              `json:"session_id"`
	Provider  string              `json:"provider"`
	Model     string              `json:"model"`
	State     string              `json:"state"`
	StartedAt int64               `json:"started_at"`
	AgentID   string              `json:"agent_id,omitempty"`
	AgentName string              `json:"agent_name,omitempty"`
	BaseSeq   int                 `json:"base_seq,omitempty"`
	Owner     string              `json:"owner,omitempty"`
	Content   string              `json:"content,omitempty"`
	Reasoning string              `json:"reasoning,omitempty"`
	Tools     []RunToolInfo       `json:"tools,omitempty"`
	Questions []agent.AskQuestion `json:"questions,omitempty"`
	EventSeq  int64               `json:"event_seq,omitempty"`
}

type RunToolInfo struct {
	CallID   string `json:"call_id,omitempty"`
	ToolName string `json:"tool_name"`
	Args     string `json:"args,omitempty"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
	Done     bool   `json:"done"`
}

type SessionMessage struct {
	ID         string         `json:"id"`
	SessionID  string         `json:"session_id"`
	Seq        int            `json:"seq"`
	Role       string         `json:"role"`
	Content    any            `json:"content"`
	Reasoning  any            `json:"reasoning,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []llm.ToolCall `json:"tool_calls,omitempty"`
	CreatedAt  string         `json:"created_at"`
}
