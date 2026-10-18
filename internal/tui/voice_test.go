package tui

import (
	"testing"

	"github.com/peggco/pegg/internal/core/config"
)

func TestRecorderCommandOverride(t *testing.T) {
	args, err := recorderCommand(&config.STTConfig{RecordCommand: "echo {output}"}, "/tmp/voice.wav")
	if err != nil {
		t.Fatalf("recorderCommand: %v", err)
	}
	if len(args) != 2 || args[0] != "echo" || args[1] != "/tmp/voice.wav" {
		t.Fatalf("args = %v", args)
	}
}

func TestRecorderCommandAppendsOutput(t *testing.T) {
	args, err := recorderCommand(&config.STTConfig{RecordCommand: "echo"}, "/tmp/voice.wav")
	if err != nil {
		t.Fatalf("recorderCommand: %v", err)
	}
	if len(args) != 2 || args[1] != "/tmp/voice.wav" {
		t.Fatalf("args = %v", args)
	}
}

func TestRecorderStreamCommandOverride(t *testing.T) {
	args, err := recorderStreamCommand(&config.STTConfig{RecordCommand: "echo"})
	if err != nil {
		t.Fatalf("recorderStreamCommand: %v", err)
	}
	if len(args) != 1 || args[0] != "echo" {
		t.Fatalf("args = %v", args)
	}
}
