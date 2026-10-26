package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/peggco/pegg/internal/stt"
	_ "github.com/peggco/pegg/internal/stt/drivers"
	_ "github.com/peggco/pegg/internal/stt/providers"
)

const maxSTTBytes = 8 << 20

func (s *Server) handleSTT(w http.ResponseWriter, r *http.Request) {
	var req STTRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Audio) == "" {
		writeError(w, http.StatusBadRequest, "audio is required")
		return
	}
	audio, err := base64.StdEncoding.DecodeString(req.Audio)
	if err != nil {
		writeError(w, http.StatusBadRequest, "audio is not valid base64")
		return
	}
	if len(audio) == 0 {
		writeError(w, http.StatusBadRequest, "audio is empty")
		return
	}
	if len(audio) > maxSTTBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "audio exceeds 8 MiB")
		return
	}
	mediaType := strings.TrimSpace(req.MediaType)
	if mediaType == "" {
		mediaType = "audio/webm"
	}
	manager := stt.New(s.cfg)
	if !manager.Configured() {
		writeError(w, http.StatusBadRequest, "speech-to-text is not configured")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	res, err := manager.Transcribe(ctx, audio, mediaType, req.FileName, "")
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, STTResponse{
		Text:     res.Text,
		Provider: res.Provider,
		Model:    res.Model,
	})
}
