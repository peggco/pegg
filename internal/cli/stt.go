package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/stt"
)

func (c *CLI) recordSTT(ctx context.Context, in io.Reader, out io.Writer) (string, error) {
	manager := stt.New(c.cfg)
	if !manager.Configured() {
		return "", errors.New("cli: speech-to-text is not configured (select an STT provider and model in Settings → General)")
	}
	file, err := os.CreateTemp("", "pegg-voice-*.wav")
	if err != nil {
		return "", fmt.Errorf("cli: create recording file: %w", err)
	}
	path := file.Name()
	_ = file.Close()
	_ = os.Chmod(path, 0600)
	defer os.Remove(path)

	args, err := cliRecorderCommand(c.cfg.STT, path)
	if err != nil {
		return "", err
	}
	cmd := exec.Command(args[0], args[1:]...)
	stdin, _ := cmd.StdinPipe()
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("cli: start recorder: %w", err)
	}
	fmt.Fprintln(out, "Listening… press Enter to stop.")
	wait := make(chan struct{})
	go func() {
		_, _ = bufio.NewReader(in).ReadString('\n')
		close(wait)
	}()
	select {
	case <-wait:
	case <-ctx.Done():
	}
	cliStopRecorder(cmd, stdin)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return "", errors.New("cli: recording is empty — check your microphone")
	}
	transcribeCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	res, err := manager.Transcribe(transcribeCtx, data, "audio/wav", "voice.wav", "")
	if err != nil {
		return "", fmt.Errorf("cli: transcribe: %w", err)
	}
	text := strings.TrimSpace(res.Text)
	if text == "" {
		return "", errors.New("cli: speech-to-text returned an empty transcript")
	}
	return text, nil
}

func cliRecorderCommand(cfg *config.STTConfig, output string) ([]string, error) {
	if cfg != nil && strings.TrimSpace(cfg.RecordCommand) != "" {
		args := strings.Fields(cfg.RecordCommand)
		if len(args) == 0 {
			return nil, errors.New("cli: stt.record_command is empty")
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
			return nil, fmt.Errorf("cli: recorder %q not found", args[0])
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
	return nil, errors.New("cli: no recorder found — install sox, arecord, or ffmpeg")
}

func cliStopRecorder(cmd *exec.Cmd, stdin io.WriteCloser) {
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
