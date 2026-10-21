package acp

import (
	"bufio"
	"context"
	"io"
	"os"
	"sync"

	json "github.com/goccy/go-json"
)

const maxStdioFrameBytes = 32 << 20

type StdioTransport struct {
	scanner *bufio.Scanner
	writer  io.Writer
	mu      sync.Mutex
}

func NewStdioTransport() *StdioTransport {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), maxStdioFrameBytes)
	return &StdioTransport{
		scanner: scanner,
		writer:  os.Stdout,
	}
}

func (t *StdioTransport) Start(ctx context.Context) (<-chan Message, error) {
	out := make(chan Message, 64)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if !t.scanner.Scan() {
				return
			}
			line := t.scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			body := make([]byte, len(line))
			copy(body, line)
			select {
			case out <- Message{Body: body}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (t *StdioTransport) Send(msg Message) error {
	data := make([]byte, 0, len(msg.Body)+1)
	data = append(data, msg.Body...)
	data = append(data, '\n')
	t.mu.Lock()
	defer t.mu.Unlock()
	_, err := t.writer.Write(data)
	return err
}

func (t *StdioTransport) Close() error { return nil }

func newStdioMessage(body json.RawMessage) Message {
	return Message{Body: body}
}
