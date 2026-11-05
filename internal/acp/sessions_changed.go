package acp

import (
	json "github.com/goccy/go-json"
)

func (s *Server) notifySessionsChanged() {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  "pegg/sessions_changed",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = s.sendRaw(raw)
}
