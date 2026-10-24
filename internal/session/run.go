package session

import (
	"encoding/json"
	"errors"
	"time"
)

func jsonString(data json.RawMessage) string {
	if len(data) == 0 {
		return "{}"
	}
	return string(data)
}

func rawJSON(value string) json.RawMessage {
	if value == "" {
		return nil
	}
	return json.RawMessage(value)
}

type RunState string

const (
	RunStateRunning  RunState = "running"
	RunStateDone     RunState = "done"
	RunStateError    RunState = "error"
	RunStateCanceled RunState = "canceled"
)

var (
	ErrRunNotFound    = errors.New("session: run not found")
	ErrRunBusy        = errors.New("session: session is already running")
	ErrRunUnsupported = errors.New("session: run store not supported by driver")
)

type Run struct {
	ID              string
	SessionID       string
	State           RunState
	OwnerInstance   string
	OwnerPID        int
	AgentID         string
	AgentName       string
	Provider        string
	Model           string
	BaseSeq         int
	CancelRequested bool
	Error           string
	StartedAt       time.Time
	UpdatedAt       time.Time
	FinishedAt      time.Time
}

type RunEvent struct {
	Seq           int64
	RunID         string
	SessionID     string
	Type          string
	AgentID       string
	ParentAgentID string
	AgentName     string
	Data          json.RawMessage
	CreatedAt     time.Time
}

type RunCommand struct {
	ID        int64
	RunID     string
	Kind      string
	Payload   json.RawMessage
	CreatedAt time.Time
}

const (
	RunCommandCancel = "cancel"
	RunCommandAnswer = "answer"
)

type RunStore interface {
	CreateRun(r Run) error
	UpdateRun(r Run) error
	GetRun(id string) (*Run, error)
	ActiveRun(sessionID string) (*Run, error)
	ListRuns() ([]Run, error)
	FinishRun(id string, state RunState, errMsg string) error
	RequestRunCancel(id string) error

	AppendRunEvents(events []RunEvent) error
	RunEvents(runID string, afterSeq int64, limit int) ([]RunEvent, error)
	RunEventsAfter(afterSeq int64, limit int) ([]RunEvent, error)
	LastRunEventSeq(runID string) (int64, error)
	LastGlobalRunEventSeq() (int64, error)

	AddRunCommand(cmd RunCommand) error
	PendingRunCommands(runID string) ([]RunCommand, error)
	ConsumeRunCommand(id int64) error

	PruneRuns(before time.Time) error
}

func (m *Manager) Runs() (RunStore, error) {
	rs, ok := m.store.(RunStore)
	if !ok {
		return nil, ErrRunUnsupported
	}
	return rs, nil
}

func (m *Manager) ActiveRun(sessionID string) (*Run, error) {
	rs, err := m.Runs()
	if err != nil {
		return nil, err
	}
	return rs.ActiveRun(sessionID)
}

const (
	RunEventStarted              = "run.started"
	RunEventState                = "run.state"
	RunEventAskResolved          = "run.ask.resolved"
	AgentEventInput              = "agent.input"
	AgentEventStarted            = "agent.started"
	AgentEventToken              = "agent.token"
	AgentEventMessage            = "agent.message"
	AgentEventToolCall           = "agent.tool_call"
	AgentEventToolResult         = "agent.tool_result"
	AgentEventUsage              = "agent.usage"
	AgentEventAsk                = "agent.ask"
	AgentEventFinished           = "agent.finished"
	AgentEventError              = "agent.error"
	AgentEventErrorMessage       = "agent.error.message"
	AgentEventErrorMessageFinish = "agent.error.message.finished"
	AgentEventSubAgentNotice     = "agent.subagent.notification"
	AgentEventCompaction         = "compaction"
)
