package session

const (
	TopicSessionCreated        = "session.created"
	TopicSessionDeleted        = "session.deleted"
	TopicSessionUpdated        = "session.updated"
	TopicSessionMessageAdded   = "session.message.added"
	TopicSessionForked         = "session.forked"
	TopicSessionReverted       = "session.reverted"
	TopicSessionRestored       = "session.restored"
	TopicSessionCurrentChanged = "session.current.changed"
	TopicSessionResume         = "session.resume"
	TopicSessionAttached       = "session.attached"
	TopicSessionCompacted      = "session.compacted"
)

type SessionResume struct {
	AgentID   string
	SessionID string
}

type SessionAttached struct {
	AgentID   string
	SessionID string
}

type SessionCreated struct {
	SessionID string
}

type SessionDeleted struct {
	SessionID string
}

type SessionUpdated struct {
	SessionID string
}

type SessionMessageAdded struct {
	SessionID string
	Message   Message
}

type SessionForked struct {
	SessionID       string
	ParentID        string
	ParentMessageID string
}

type SessionReverted struct {
	SessionID   string
	ToMessageID string
	SnapshotID  string
}

type SessionRestored struct {
	SessionID  string
	SnapshotID string
}

type SessionCurrentChanged struct {
	SessionID string
}

type SessionCompacted struct {
	SessionID       string
	ParentSessionID string
	Strategy        string
	MessageCount    int
}

const (
	TopicRunStarted     = "run.started"
	TopicRunFinished    = "run.finished"
	TopicRunAskResolved = "run.ask.resolved"
	TopicSessionChanged = "session.changed"
)

type RunStarted struct {
	RunID     string
	SessionID string
	AgentID   string
	AgentName string
	Provider  string
	Model     string
	BaseSeq   int
}

type RunFinished struct {
	RunID     string
	SessionID string
	AgentID   string
	State     RunState
	Error     string
}

type RunAskResolved struct {
	RunID     string
	SessionID string
	AgentID   string
	Answers   map[string]string
}

type SessionChanged struct {
	SessionID string
	Kind      string
	Origin    string
}
