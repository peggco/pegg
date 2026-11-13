package mcp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type Duplex interface {
	Connect(ctx context.Context) error
	Write(data []byte) error
	ReadFrame() ([]byte, error)
	Close() error
}

type ProtocolVersionSetter interface {
	SetProtocolVersion(version string)
}

type StreamStarter interface {
	StartStream()
}

type FrameDecoder struct {
	br     *bufio.Reader
	kind   int
	toRead int
}

const (
	frameNewline = iota
	frameLength
)

const maxFrameSize = 16 << 20

func NewFrameDecoder(r io.Reader) *FrameDecoder {
	return &FrameDecoder{br: bufio.NewReader(r), kind: frameNewline}
}

func (d *FrameDecoder) Decode() ([]byte, error) {
	for {
		switch d.kind {
		case frameLength:
			return d.decodeContentLength()
		default:
			line, err := d.br.ReadBytes('\n')
			if err != nil && len(line) == 0 {
				return nil, err
			}
			trimmed := bytes.TrimSpace(bytes.TrimRight(line, "\r\n"))
			if strings.HasPrefix(string(trimmed), "Content-Length:") {
				raw, perr := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(string(trimmed), "Content-Length:")))
				if perr != nil || raw <= 0 {
					return nil, fmt.Errorf("%w: invalid Content-Length", ErrProtocol)
				}
				if raw > maxFrameSize {
					return nil, fmt.Errorf("%w: frame too large", ErrProtocol)
				}
				d.kind = frameLength
				d.toRead = raw
				continue
			}
			if len(trimmed) == 0 {
				continue
			}
			return trimmed, nil
		}
	}
}

func (d *FrameDecoder) decodeContentLength() ([]byte, error) {
	peek, err := d.br.Peek(2)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	if len(peek) >= 1 && (peek[0] == '\r' || peek[0] == '\n') {
		_, _ = d.br.ReadByte()
		if len(peek) >= 2 && (peek[1] == '\r' || peek[1] == '\n') {
			_, _ = d.br.ReadByte()
		}
	}

	body := make([]byte, d.toRead)
	if _, err := io.ReadFull(d.br, body); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	d.kind = frameNewline
	return body, nil
}
