package changejournal

import "time"

type Op string

const (
	OpAdd    Op = "add"
	OpModify Op = "modify"
	OpDelete Op = "delete"
)

type Status string

const (
	StatusRestored        Status = "restored"
	StatusDeleted         Status = "deleted"
	StatusSkipped         Status = "skipped"
	StatusConflict        Status = "conflict"
	StatusNotTracked      Status = "not_tracked"
	StatusAlreadyReverted Status = "already_reverted"
	StatusMissingBackup   Status = "missing_backup"
)

type Entry struct {
	ID            string     `json:"id"`
	SessionID     string     `json:"session_id"`
	MessageID     string     `json:"message_id,omitempty"`
	ToolCallID    string     `json:"tool_call_id,omitempty"`
	Path          string     `json:"path"`
	Op            Op         `json:"op"`
	BeforeHash    string     `json:"before_hash,omitempty"`
	AfterHash     string     `json:"after_hash,omitempty"`
	BackupPath    string     `json:"backup_path,omitempty"`
	ForwardBackup string     `json:"forward_backup,omitempty"`
	Size          int64      `json:"size,omitempty"`
	Tracked       bool       `json:"tracked"`
	Reason        string     `json:"reason,omitempty"`
	Seq           int64      `json:"seq"`
	CreatedAt     time.Time  `json:"created_at"`
	RevertedAt    *time.Time `json:"reverted_at,omitempty"`
	RevertID      string     `json:"revert_id,omitempty"`
	RestoredHash  string     `json:"restored_hash,omitempty"`
	Conflict      string     `json:"conflict,omitempty"`
	UndoConflict  string     `json:"undo_conflict,omitempty"`
}

type Change struct {
	Path   string `json:"path"`
	Op     Op     `json:"op"`
	Status Status `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type Result struct {
	SessionID string   `json:"session_id"`
	Action    string   `json:"action,omitempty"`
	RevertID  string   `json:"revert_id,omitempty"`
	Files     []Change `json:"files"`
	Restored  int      `json:"restored"`
	Deleted   int      `json:"deleted"`
	Conflicts int      `json:"conflicts"`
	Skipped   int      `json:"skipped"`
}

const (
	ActionRevert = "revert"
	ActionUndo   = "undo"
)

const (
	TopicChangeJournal         = "changejournal"
	TopicChangeJournalReverted = "changejournal.reverted"
	TopicChangeJournalRestored = "changejournal.restored"
)

func (r *Result) add(c Change) {
	r.Files = append(r.Files, c)
	switch c.Status {
	case StatusRestored:
		r.Restored++
	case StatusDeleted:
		r.Deleted++
	case StatusConflict, StatusMissingBackup:
		r.Conflicts++
	default:
		r.Skipped++
	}
}
