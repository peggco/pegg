package server

import (
	"encoding/json"
	"net/http"

	"github.com/peggco/pegg/internal/live"
	"github.com/peggco/pegg/internal/session"
)

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	if s.runs == nil {
		writeJSON(w, http.StatusOK, map[string]any{"runs": []any{}})
		return
	}
	runs, err := s.runs.ActiveRuns()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(runs))
	for i := range runs {
		out = append(out, runInfo(&runs[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": out})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	if s.runs == nil {
		writeError(w, http.StatusNotFound, "run store unavailable")
		return
	}
	run, err := s.runs.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, runInfo(run))
}

func (s *Server) handleCancelRun(w http.ResponseWriter, r *http.Request) {
	if s.runs == nil {
		writeError(w, http.StatusNotFound, "run store unavailable")
		return
	}
	id := r.PathValue("id")
	if err := s.runs.Cancel(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run_id": id, "state": string(session.RunStateCanceled)})
}

func (s *Server) handleAnswerRun(w http.ResponseWriter, r *http.Request) {
	if s.runs == nil {
		writeError(w, http.StatusNotFound, "run store unavailable")
		return
	}
	var req struct {
		Answers map[string]string `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Answers) == 0 {
		writeError(w, http.StatusBadRequest, "answers are required")
		return
	}
	id := r.PathValue("id")
	if err := s.runs.Answer(id, req.Answers); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run_id": id, "answered": true})
}

func (s *Server) handleSessionStream(w http.ResponseWriter, r *http.Request) {
	if s.runs == nil {
		writeError(w, http.StatusNotFound, "run store unavailable")
		return
	}
	sessionID := r.PathValue("id")
	follow := r.URL.Query().Get("follow") == "true"

	active, err := s.runs.Active(sessionID)
	if err != nil && !follow {
		writeError(w, http.StatusNotFound, "session is not running")
		return
	}

	sse, err := NewSSEWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sse not supported")
		return
	}
	defer sse.Close()

	if s.live == nil {
		if active == nil {
			writeError(w, http.StatusNotFound, "live hub unavailable")
			return
		}
		if err := sse.WriteEvent("run", runInfo(active)); err != nil {
			return
		}
		_ = s.runs.Watch(r.Context(), active.ID, 0, func(e session.RunEvent) error {
			return sse.WriteEvent(e.Type, json.RawMessage(e.Data))
		})
		return
	}

	snapshot, since, err := s.live.SnapshotSession(sessionID, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sub := s.live.SubscribeSession(sessionID, true, since)
	defer sub.Close()

	writeEvent := func(e live.Event) bool {
		switch e.Type {
		case live.EventRunStarted:
			return sse.WriteEvent("run", runInfo(&e.Run)) == nil
		case live.EventRunFinished:
			return sse.WriteEvent("end", map[string]any{
				"run_id": e.Run.ID,
				"state":  string(e.State),
				"error":  e.Error,
			}) == nil
		default:
			return sse.WriteEvent(e.Journal.Type, json.RawMessage(e.Journal.Data)) == nil
		}
	}
	for i := range snapshot {
		if !writeEvent(snapshot[i]) {
			return
		}
		if snapshot[i].Type == live.EventRunFinished && !follow {
			return
		}
	}
	if active != nil && !follow {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-sub.C:
			if !ok {
				return
			}
			if !writeEvent(e) {
				return
			}
			if e.Type == live.EventRunFinished && !follow {
				return
			}
		}
	}
}
