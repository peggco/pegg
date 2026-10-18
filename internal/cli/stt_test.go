package cli

import (
	"testing"

	"github.com/peggco/pegg/internal/core/config"
)

func TestCLIRecorderCommandOverride(t *testing.T) {
	args, err := cliRecorderCommand(&config.STTConfig{RecordCommand: "echo {output}"}, "/tmp/voice.wav")
	if err != nil {
		t.Fatalf("recorder: %v", err)
	}
	if len(args) != 2 || args[1] != "/tmp/voice.wav" {
		t.Fatalf("args = %v", args)
	}
}
