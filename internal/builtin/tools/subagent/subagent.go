package subagent

import (
	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/session"
)

type HistoryReader interface {
	Messages(sessionID string) ([]session.Message, error)
	Bus() event.Bus
}

var sessionReader HistoryReader

func SubAgentTools(reader HistoryReader) {
	sessionReader = reader
	if reader != nil {
		store.subscribe(reader.Bus())
	}
	tool.Default().Register(subAgentTool())
	tool.Default().Register(subAgentsStatusTool())
}
