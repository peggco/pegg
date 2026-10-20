package acp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
)

type recording struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	path  string
}

func (s *Server) handleSTTRecordStart(ctx context.Context, params json.RawMessage) (any, error) {
	s.recMu.Lock()
	defer s.recMu.Unlock()
	if s.rec != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "a recording is already in progress"}
	}
	if s.cfg == nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "config unavailable"}
	}
	manager := stt.New(s.cfg)
	if !manager.Configured() {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "speech-to-text is not configured (select an STT provider and model in Settings → General)"}
	}

	file, err := os.CreateTemp("", "pegg-voice-*.wav")
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "could not create recording file: " + err.Error()}
	}
	path := file.Name()
	_ = file.Close()
	_ = os.Chmod(path, 0o600)

	args, err := acpRecorderCommand(s.cfg.STT, path)
	if err != nil {
		_ = os.Remove(path)
		return nil, &RPCError{Code: ErrCodeInternal, Message: err.Error()}
	}
	cmd := exec.Command(args[0], args[1:]...)
	stdin, _ := cmd.StdinPipe()
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = os.Remove(path)
		return nil, &RPCError{Code: ErrCodeInternal, Message: "could not start recorder: " + err.Error()}
	}
	s.rec = &recording{cmd: cmd, stdin: stdin, path: path}
	return map[string]any{"recording": true}, nil
}

func (s *Server) handleSTTRecordStop(ctx context.Context, params json.RawMessage) (any, error) {
	s.recMu.Lock()
	rec := s.rec
	s.rec = nil
	s.recMu.Unlock()
	if rec == nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "no recording is in progress"}
	}
	stopRecorder(rec.cmd, rec.stdin)
	defer os.Remove(rec.path)

	data, err := os.ReadFile(rec.path)
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "could not read recording: " + err.Error()}
	}
	if len(data) == 0 {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "recording is empty — check your microphone"}
	}
	if len(data) > maxSTTBytes {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "recording exceeds 8 MiB"}
	}
	if s.cfg == nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "config unavailable"}
	}
	manager := stt.New(s.cfg)
	if !manager.Configured() {
		return nil, &RPCError{Code: ErrCodeInternal, Message: "speech-to-text is not configured"}
	}
	transcribeCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	res, err := manager.Transcribe(transcribeCtx, data, "audio/wav", "voice.wav", "")
	if err != nil {
		return nil, &RPCError{Code: ErrCodeInternal, Message: err.Error()}
	}
	return STTTranscribeResult{Text: res.Text, Provider: res.Provider, Model: res.Model}, nil
}

func (s *Server) cleanupRecording() {
	s.recMu.Lock()
	rec := s.rec
	s.rec = nil
	s.recMu.Unlock()
	if rec == nil {
		return
	}
	stopRecorder(rec.cmd, rec.stdin)
	_ = os.Remove(rec.path)
}

func acpRecorderCommand(cfg *config.STTConfig, output string) ([]string, error) {
	if cfg != nil && strings.TrimSpace(cfg.RecordCommand) != "" {
		args := strings.Fields(cfg.RecordCommand)
		if len(args) == 0 {
			return nil, errors.New("stt.record_command is empty")
		}
		replaced := false
		for i := range args {
			if strings.Contains(args[i], "{output}") {
				args[i] = strings.ReplaceAll(args[i], "{output}", output)
				replaced = true
			}
		}
		if !replaced {
			args = append(args, output)
		}
		if _, err := exec.LookPath(args[0]); err != nil {
			return nil, fmt.Errorf("recorder %q not found", args[0])
		}
		return args, nil
	}
	candidates := [][]string{
		{"sox", "-q", "-d", "-r", "16000", "-c", "1", "-b", "16", output},
	}
	if runtime.GOOS == "linux" {
		candidates = append(candidates, []string{"arecord", "-q", "-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "wav", output})
	}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates, []string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "avfoundation", "-i", ":0", "-ar", "16000", "-ac", "1", output})
	case "windows":
		candidates = append(candidates, []string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "dshow", "-i", "audio=Microphone", "-ar", "16000", "-ac", "1", output})
	default:
		candidates = append(candidates,
			[]string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "pulse", "-i", "default", "-ar", "16000", "-ac", "1", output},
			[]string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "alsa", "-i", "default", "-ar", "16000", "-ac", "1", output},
		)
	}
	for _, args := range candidates {
		if _, err := exec.LookPath(args[0]); err == nil {
			return args, nil
		}
	}
	return nil, errors.New("no recorder found — install sox, arecord, or ffmpeg")
}

func stopRecorder(cmd *exec.Cmd, stdin io.WriteCloser) {
	if stdin != nil {
		_, _ = io.WriteString(stdin, "q\n")
		_ = stdin.Close()
	}
	if cmd == nil || cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		_ = cmd.Process.Kill()
	} else {
		_ = cmd.Process.Signal(os.Interrupt)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}
