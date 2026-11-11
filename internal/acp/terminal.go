package acp

import (
	"context"
	"fmt"
	"time"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/agent/tool"
)

type TerminalOutputResult struct {
	Meta       Meta                `json:"_meta,omitempty"`
	Output     string              `json:"output"`
	Truncated  bool                `json:"truncated"`
	ExitStatus *TerminalExitStatus `json:"exitStatus,omitempty"`
}

type createTerminalParams struct {
	Meta            Meta      `json:"_meta,omitempty"`
	SessionID       SessionId `json:"sessionId"`
	Command         string    `json:"command"`
	Args            []string  `json:"args,omitempty"`
	Env             []EnvVar  `json:"env,omitempty"`
	Cwd             string    `json:"cwd,omitempty"`
	OutputByteLimit *int      `json:"outputByteLimit,omitempty"`
}

type terminalIDParams struct {
	Meta       Meta       `json:"_meta,omitempty"`
	SessionID  SessionId  `json:"sessionId"`
	TerminalID TerminalId `json:"terminalId"`
}

func (s *Server) clientSupportsTerminal() bool {
	caps := s.clientCapabilities()
	return caps != nil && caps.Terminal
}

func (s *Server) CreateTerminal(ctx context.Context, sessionID string, command string, args []string, env []EnvVar, cwd string, outputByteLimit *int) (TerminalId, error) {
	params := createTerminalParams{
		SessionID:       SessionId(sessionID),
		Command:         command,
		Args:            args,
		Env:             env,
		Cwd:             cwd,
		OutputByteLimit: outputByteLimit,
	}
	raw, err := s.client.call(ctx, "terminal/create", params, s.client.timeout)
	if err != nil {
		return "", err
	}
	var resp struct {
		Result struct {
			TerminalID TerminalId `json:"terminalId"`
		} `json:"result"`
		Error *RPCError `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", resp.Error
	}
	return resp.Result.TerminalID, nil
}

func (s *Server) TerminalOutput(ctx context.Context, sessionID string, terminalID TerminalId) (*TerminalOutputResult, error) {
	raw, err := s.client.call(ctx, "terminal/output", terminalIDParams{SessionID: SessionId(sessionID), TerminalID: terminalID}, s.client.timeout)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result TerminalOutputResult `json:"result"`
		Error  *RPCError            `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return &resp.Result, nil
}

func (s *Server) TerminalWaitForExit(ctx context.Context, sessionID string, terminalID TerminalId) (*TerminalExitStatus, error) {
	raw, err := s.client.call(ctx, "terminal/wait_for_exit", terminalIDParams{SessionID: SessionId(sessionID), TerminalID: terminalID}, 0)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result TerminalExitStatus `json:"result"`
		Error  *RPCError          `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return &resp.Result, nil
}

func (s *Server) TerminalKill(ctx context.Context, sessionID string, terminalID TerminalId) error {
	_, err := s.client.call(ctx, "terminal/kill", terminalIDParams{SessionID: SessionId(sessionID), TerminalID: terminalID}, s.client.timeout)
	return err
}

func (s *Server) TerminalRelease(ctx context.Context, sessionID string, terminalID TerminalId) error {
	_, err := s.client.call(ctx, "terminal/release", terminalIDParams{SessionID: SessionId(sessionID), TerminalID: terminalID}, s.client.timeout)
	return err
}

type acpTerminalRunner struct {
	server    *Server
	sessionID string
	cwd       string
}

func (r *acpTerminalRunner) RunTerminal(ctx context.Context, command string, timeout time.Duration) (string, bool, error) {
	if !r.server.clientSupportsTerminal() {
		return "", false, nil
	}
	termID, err := r.server.CreateTerminal(ctx, r.sessionID, "sh", []string{"-c", command}, nil, r.cwd, nil)
	if err != nil {
		return "", false, nil
	}
	defer func() {
		_ = r.server.TerminalRelease(context.Background(), r.sessionID, termID)
	}()

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	done := make(chan *TerminalExitStatus, 1)
	go func() {
		st, _ := r.server.TerminalWaitForExit(waitCtx, r.sessionID, termID)
		done <- st
	}()

	writer := tool.OutputWriter(ctx, "stdout")
	emitted := 0
	stream := func() {
		res, err := r.server.TerminalOutput(context.Background(), r.sessionID, termID)
		if err != nil || res == nil {
			return
		}
		if res.Truncated {
			emitted = 0
		}
		if len(res.Output) <= emitted {
			return
		}
		_, _ = writer.Write([]byte(res.Output[emitted:]))
		emitted = len(res.Output)
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var exit *TerminalExitStatus
	finished := false
	for !finished {
		select {
		case exit = <-done:
			finished = true
		case <-ticker.C:
			stream()
		case <-waitCtx.Done():
			_ = r.server.TerminalKill(context.Background(), r.sessionID, termID)
			exit = <-done
			finished = true
		}
	}
	stream()

	out, err := r.server.TerminalOutput(context.Background(), r.sessionID, termID)
	if err != nil {
		return "", true, err
	}
	result := ""
	if exit != nil && exit.ExitCode != nil {
		result = fmt.Sprintf("Exit code: %d\n", *exit.ExitCode)
	}
	if out != nil && out.Output != "" {
		result += "stdout:\n" + out.Output + "\n"
	}
	return result, true, nil
}
