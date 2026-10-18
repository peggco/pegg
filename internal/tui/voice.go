package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/stt"
)

type voiceController struct {
	mu sync.Mutex

	recording    bool
	transcribing bool
	streaming    bool
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	file         string
	started      time.Time
	timer        *time.Timer
	cancel       context.CancelFunc
	streamDone   chan error
	streamPCM    bytes.Buffer
	preview      string
	finalText    string
	cancelled    bool

	pendingText    string
	pendingPreview string
	pendingErr     string
}

func (a *App) toggleVoice() {
	v := &a.voice
	v.mu.Lock()
	recording, transcribing := v.recording, v.transcribing
	v.mu.Unlock()
	switch {
	case recording:
		a.stopVoice(false)
	case transcribing:
		a.cancelVoice()
	default:
		a.startVoice()
	}
}

func (a *App) cancelVoiceIfActive() bool {
	v := &a.voice
	v.mu.Lock()
	recording, transcribing := v.recording, v.transcribing
	v.mu.Unlock()
	if !recording && !transcribing {
		return false
	}
	a.cancelVoice()
	return true
}

func (a *App) startVoice() {
	cfg := a.currentConfig()
	svc := stt.New(cfg)
	if !svc.Configured() {
		a.voiceError("speech-to-text is not configured — choose a provider and model in Settings → General")
		return
	}
	v := &a.voice
	v.mu.Lock()
	if v.recording || v.transcribing {
		v.mu.Unlock()
		return
	}
	v.mu.Unlock()

	if svc.SupportsStream() {
		a.startVoiceStream(cfg, svc)
		return
	}
	a.startVoiceFile(cfg, svc)
}

func (a *App) startVoiceFile(cfg *config.Config, svc *stt.Manager) {
	file, err := os.CreateTemp("", "pegg-voice-*.wav")
	if err != nil {
		a.voiceError("voice: create recording file: " + err.Error())
		return
	}
	path := file.Name()
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		a.voiceError("voice: secure recording file: " + err.Error())
		return
	}
	_ = file.Close()

	args, err := recorderCommand(cfg.STT, path)
	if err != nil {
		_ = os.Remove(path)
		a.voiceError(err.Error())
		return
	}
	cmd := exec.Command(args[0], args[1:]...)
	stdin, _ := cmd.StdinPipe()
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		_ = os.Remove(path)
		a.voiceError("voice: start recorder: " + err.Error())
		return
	}
	a.beginRecording(cfg, cmd, stdin, path, false)
}

func (a *App) startVoiceStream(cfg *config.Config, svc *stt.Manager) {
	args, err := recorderStreamCommand(cfg.STT)
	if err != nil {
		a.voiceError(err.Error())
		return
	}
	cmd := exec.Command(args[0], args[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.voiceError("voice: capture recorder output: " + err.Error())
		return
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		a.voiceError("voice: start recorder: " + err.Error())
		return
	}

	v := &a.voice
	ctx, cancel := context.WithCancel(a.ctx)
	v.mu.Lock()
	v.streaming = true
	v.cancelled = false
	v.streamPCM.Reset()
	v.finalText = ""
	v.preview = ""
	v.cancel = cancel
	v.streamDone = make(chan error, 1)
	done := v.streamDone
	v.mu.Unlock()

	a.beginRecording(cfg, cmd, nil, "", true)

	go func() {
		defer cancel()
		tee := io.TeeReader(stdout, &v.streamPCM)
		err := svc.Stream(ctx, tee, func(ev stt.StreamEvent) error {
			text := strings.TrimSpace(ev.Text)
			v.mu.Lock()
			if ev.Final {
				if text != "" {
					if v.finalText != "" {
						v.finalText += " "
					}
					v.finalText += text
					if v.pendingText != "" {
						v.pendingText += " "
					}
					v.pendingText += text
				}
				v.preview = ""
			} else {
				v.preview = text
			}
			v.mu.Unlock()
			if a.bus != nil {
				a.bus.Publish(event.TopicSTTPartial, event.STTEvent{
					Source: "tui",
					Text:   text,
					Final:  ev.Final,
				})
			}
			a.requestRedraw()
			return nil
		})

		v.mu.Lock()
		cancelled := v.cancelled
		hadFinal := v.finalText != ""
		pcm := append([]byte(nil), v.streamPCM.Bytes()...)
		v.mu.Unlock()

		if err != nil && !cancelled && !errors.Is(err, context.Canceled) {
			if len(pcm) > 0 {
				fallbackCtx, fallbackCancel := context.WithTimeout(context.Background(), 90*time.Second)
				res, ferr := svc.Transcribe(fallbackCtx, stt.WAVFromPCM(pcm, 16000), "audio/wav", "voice.wav", "")
				fallbackCancel()
				if ferr == nil && res.Text != "" {
					v.mu.Lock()
					v.pendingText = res.Text
					v.finalText = res.Text
					v.preview = ""
					v.mu.Unlock()
					hadFinal = true
				} else {
					a.voiceError("voice: " + err.Error())
				}
			} else {
				a.voiceError("voice: " + err.Error())
			}
		}

		v.mu.Lock()
		if !hadFinal && v.preview != "" {
			v.pendingText = v.preview
		}
		v.preview = ""
		v.transcribing = false
		v.streaming = false
		v.recording = false
		v.cancel = nil
		started := v.started
		v.mu.Unlock()
		if a.bus != nil && hadFinal {
			a.bus.Publish(event.TopicSTTFinished, event.STTEvent{
				Source:     "tui",
				DurationMS: time.Since(started).Milliseconds(),
			})
		}
		a.applyVoiceState("", 0)
		done <- err
		a.requestRedraw()
	}()
}

func (a *App) beginRecording(cfg *config.Config, cmd *exec.Cmd, stdin io.WriteCloser, file string, streaming bool) {
	duration := time.Duration(cfg.STT.MaxDurationValue()) * time.Second
	v := &a.voice
	v.mu.Lock()
	v.recording = true
	v.transcribing = false
	v.streaming = streaming
	v.cmd = cmd
	v.stdin = stdin
	v.file = file
	v.started = time.Now()
	v.pendingErr = ""
	v.timer = time.AfterFunc(duration, func() { a.stopVoice(false) })
	v.mu.Unlock()

	if a.bus != nil {
		a.bus.Publish(event.TopicSTTStarted, event.STTEvent{Source: "tui"})
	}
	a.applyVoiceState("recording", 0)
	a.requestRedraw()
}

func (a *App) stopVoice(cancel bool) {
	v := &a.voice
	v.mu.Lock()
	if !v.recording {
		v.mu.Unlock()
		return
	}
	v.recording = false
	if v.timer != nil {
		v.timer.Stop()
		v.timer = nil
	}
	cmd, stdin, file := v.cmd, v.stdin, v.file
	streaming := v.streaming
	done := v.streamDone
	v.cmd, v.stdin = nil, nil
	if cancel {
		v.cancelled = true
		v.transcribing = false
		v.file = ""
		if v.cancel != nil {
			v.cancel()
		}
	}
	started := v.started
	v.mu.Unlock()

	if streaming {
		if cancel {
			if cmd != nil {
				stopRecorder(cmd, stdin)
			}
			a.applyVoiceState("", 0)
			a.requestRedraw()
			return
		}
		v.mu.Lock()
		v.transcribing = true
		v.mu.Unlock()
		a.applyVoiceState("transcribing", 0)
		a.requestRedraw()
		if cmd != nil {
			stopRecorder(cmd, stdin)
		}
		if done != nil {
			select {
			case <-done:
			case <-time.After(20 * time.Second):
				v.mu.Lock()
				if v.cancel != nil {
					v.cancel()
				}
				v.mu.Unlock()
			}
		}
		_ = started
		return
	}

	if cancel {
		if cmd != nil {
			stopRecorder(cmd, stdin)
		}
		_ = os.Remove(file)
		a.applyVoiceState("", 0)
		a.requestRedraw()
		return
	}

	v.mu.Lock()
	v.transcribing = true
	v.file = ""
	v.mu.Unlock()
	a.applyVoiceState("transcribing", 0)
	a.requestRedraw()
	if cmd != nil {
		stopRecorder(cmd, stdin)
	}

	go func() {
		defer func() {
			_ = os.Remove(file)
			v.mu.Lock()
			v.transcribing = false
			v.cancel = nil
			v.mu.Unlock()
			a.applyVoiceState("", 0)
			a.requestRedraw()
		}()

		data, err := os.ReadFile(file)
		if err != nil || len(data) == 0 {
			a.voiceError("voice: recording is empty — check your microphone")
			return
		}

		ctx, cancelFn := context.WithTimeout(context.Background(), 90*time.Second)
		v.mu.Lock()
		v.cancel = cancelFn
		v.mu.Unlock()
		defer cancelFn()

		svc := stt.New(a.currentConfig())
		res, err := svc.Transcribe(ctx, data, "audio/wav", "voice.wav", "")
		if err != nil {
			a.voiceError("voice: " + err.Error())
			return
		}
		if a.bus != nil {
			a.bus.Publish(event.TopicSTTFinished, event.STTEvent{
				Source:     "tui",
				DurationMS: time.Since(started).Milliseconds(),
			})
		}
		v.mu.Lock()
		v.pendingText = res.Text
		v.pendingErr = ""
		v.mu.Unlock()
		a.requestRedraw()
	}()
}

func (a *App) cancelVoice() {
	v := &a.voice
	v.mu.Lock()
	recording := v.recording
	transcribing := v.transcribing
	streaming := v.streaming
	cmd, stdin, file := v.cmd, v.stdin, v.file
	cancelFn := v.cancel
	v.mu.Unlock()

	if recording {
		a.stopVoice(true)
		return
	}
	if transcribing || streaming {
		if cancelFn != nil {
			cancelFn()
		}
		if cmd != nil {
			stopRecorder(cmd, stdin)
		}
		v.mu.Lock()
		v.transcribing = false
		v.streaming = false
		v.recording = false
		v.cancel = nil
		v.preview = ""
		if file != "" {
			_ = os.Remove(file)
		}
		v.mu.Unlock()
	}
	a.applyVoiceState("", 0)
	a.requestRedraw()
}

func (a *App) tickVoice() {
	v := &a.voice
	v.mu.Lock()
	recording := v.recording
	started := v.started
	text := v.pendingText
	preview := v.preview
	errMsg := v.pendingErr
	v.pendingText = ""
	v.pendingErr = ""
	v.mu.Unlock()

	if recording {
		a.applyVoiceState("recording", int(time.Since(started).Seconds()))
	}
	if a.home != nil {
		a.home.SetVoicePreview(preview)
	}
	if text == "" && errMsg == "" {
		if preview != "" {
			a.requestRedraw()
		}
		return
	}
	if text != "" && a.home != nil {
		a.chatMu.Lock()
		a.home.Input().InsertText(text)
		a.home.Input().Focus()
		a.refreshMentionItemsLocked()
		a.chatMu.Unlock()
		a.errorMsg = ""
	}
	if errMsg != "" {
		a.errorMsg = errMsg
	}
	a.requestRedraw()
}

func (a *App) applyVoiceState(state string, elapsed int) {
	if a.home != nil {
		a.home.SetVoiceState(state, elapsed)
	}
}

func (a *App) voiceError(msg string) {
	v := &a.voice
	v.mu.Lock()
	v.pendingErr = msg
	v.mu.Unlock()
	if a.bus != nil {
		a.bus.Publish(event.TopicSTTError, event.STTEvent{Source: "tui", Message: msg})
	}
	a.requestRedraw()
}

func (a *App) currentConfig() *config.Config {
	if a.deps.Config != nil {
		return a.deps.Config
	}
	cfg, err := config.Load()
	if err != nil {
		return nil
	}
	return cfg
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

func recorderCommand(cfg *config.STTConfig, output string) ([]string, error) {
	if cfg != nil && strings.TrimSpace(cfg.RecordCommand) != "" {
		args := strings.Fields(cfg.RecordCommand)
		if len(args) == 0 {
			return nil, errors.New("voice: record_command is empty")
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
			return nil, fmt.Errorf("voice: recorder %q not found", args[0])
		}
		return args, nil
	}

	for _, c := range fileRecorderCandidates(output) {
		if _, err := exec.LookPath(c[0]); err == nil {
			return c, nil
		}
	}
	return nil, errors.New("voice: no recorder found — install sox, arecord, or ffmpeg, or set stt.record_command")
}

func recorderStreamCommand(cfg *config.STTConfig) ([]string, error) {
	if cfg != nil && strings.TrimSpace(cfg.RecordCommand) != "" {
		args := strings.Fields(cfg.RecordCommand)
		if len(args) == 0 {
			return nil, errors.New("voice: record_command is empty")
		}
		if _, err := exec.LookPath(args[0]); err != nil {
			return nil, fmt.Errorf("voice: recorder %q not found", args[0])
		}
		return args, nil
	}
	for _, c := range streamRecorderCandidates() {
		if _, err := exec.LookPath(c[0]); err == nil {
			return c, nil
		}
	}
	return nil, errors.New("voice: no streaming recorder found — install sox, arecord, or ffmpeg")
}

func fileRecorderCandidates(output string) [][]string {
	var out [][]string
	out = append(out, []string{"sox", "-q", "-d", "-r", "16000", "-c", "1", "-b", "16", output})
	if runtime.GOOS == "linux" {
		out = append(out, []string{"arecord", "-q", "-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "wav", output})
	}
	switch runtime.GOOS {
	case "darwin":
		out = append(out, []string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "avfoundation", "-i", ":0", "-ar", "16000", "-ac", "1", output})
	case "windows":
		out = append(out, []string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "dshow", "-i", "audio=Microphone", "-ar", "16000", "-ac", "1", output})
	default:
		out = append(out, []string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "pulse", "-i", "default", "-ar", "16000", "-ac", "1", output})
		out = append(out, []string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "alsa", "-i", "default", "-ar", "16000", "-ac", "1", output})
	}
	return out
}

func streamRecorderCandidates() [][]string {
	var out [][]string
	out = append(out, []string{"sox", "-q", "-d", "-r", "16000", "-c", "1", "-b", "16", "-e", "signed-integer", "-t", "raw", "-"})
	if runtime.GOOS == "linux" {
		out = append(out, []string{"arecord", "-q", "-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "raw", "-"})
	}
	switch runtime.GOOS {
	case "darwin":
		out = append(out, []string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "avfoundation", "-i", ":0", "-ar", "16000", "-ac", "1", "-f", "s16le", "-"})
	case "windows":
		out = append(out, []string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "dshow", "-i", "audio=Microphone", "-ar", "16000", "-ac", "1", "-f", "s16le", "-"})
	default:
		out = append(out, []string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "pulse", "-i", "default", "-ar", "16000", "-ac", "1", "-f", "s16le", "-"})
		out = append(out, []string{"ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "alsa", "-i", "default", "-ar", "16000", "-ac", "1", "-f", "s16le", "-"})
	}
	return out
}
