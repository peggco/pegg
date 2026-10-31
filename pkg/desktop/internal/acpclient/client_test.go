package acpclient

import (
	"bufio"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestReadFrame(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("{\"a\":1}\r\n{\"b\":2}\n"))
	first, err := readFrame(r, 1024)
	if err != nil || string(first) != `{"a":1}` {
		t.Fatalf("first frame = %q, %v", first, err)
	}
	second, err := readFrame(r, 1024)
	if err != nil || string(second) != `{"b":2}` {
		t.Fatalf("second frame = %q, %v", second, err)
	}
}

func TestReadFrameTooLarge(t *testing.T) {
	r := bufio.NewReader(strings.NewReader(strings.Repeat("x", 64) + "\n"))
	_, err := readFrame(r, 16)
	if !errors.Is(err, ErrFrameTooBig) {
		t.Fatalf("expected ErrFrameTooBig, got %v", err)
	}
}

func TestNormID(t *testing.T) {
	if got := normID(json.RawMessage(`7`)); got != "7" {
		t.Fatalf("number id = %q", got)
	}
	if got := normID(json.RawMessage(`"abc"`)); got != "abc" {
		t.Fatalf("string id = %q", got)
	}
}
