package middlewares

import (
	"strings"
	"testing"
)

func testRepetitionConfig() repetitionConfig {
	return repetitionConfig{
		MinUnitChars: 12,
		MaxUnitChars: 400,
		MinRepeats:   5,
		WindowChars:  8192,
	}
}

func TestDetectRepeatedTextThinkingSentenceLoop(t *testing.T) {
	text := strings.Repeat("I should check the file. ", 100)
	hit, ok := detectRepeatedText("thinking", text, testRepetitionConfig())
	if !ok {
		t.Fatal("expected repetition hit")
	}
	if hit.Source != "thinking" {
		t.Fatalf("source = %q, want thinking", hit.Source)
	}
	if hit.Repeats < 5 {
		t.Fatalf("repeats = %d, want >= 5", hit.Repeats)
	}
	if !strings.Contains(hit.Unit, "check the file") {
		t.Fatalf("unit = %q, want sentence preview", hit.Unit)
	}
}

func TestDetectRepeatedTextThreeWordSentence(t *testing.T) {
	text := strings.TrimSpace(strings.Repeat("let us check ", 80))
	hit, ok := detectRepeatedText("thinking", text, testRepetitionConfig())
	if !ok {
		t.Fatal("expected repetition hit for a repeated three-word sentence")
	}
	if hit.Kind != "words" {
		t.Fatalf("kind = %q, want words", hit.Kind)
	}
	if hit.Repeats < 5 {
		t.Fatalf("repeats = %d, want >= 5", hit.Repeats)
	}
}

func TestDetectRepeatedTextLineLoop(t *testing.T) {
	line := "func handleRequest() error {"
	text := strings.Repeat(line+"\n", 8)
	hit, ok := detectRepeatedText("content", text, testRepetitionConfig())
	if !ok {
		t.Fatal("expected line repetition hit")
	}
	if hit.Kind != "line" {
		t.Fatalf("kind = %q, want line", hit.Kind)
	}
}

func TestDetectRepeatedTextSentenceLoop(t *testing.T) {
	text := "We need to verify the parser. " + strings.Repeat("The same sentence repeats here. ", 7)
	hit, ok := detectRepeatedText("thinking", text, testRepetitionConfig())
	if !ok {
		t.Fatal("expected sentence repetition hit")
	}
	if hit.Kind != "sentence" {
		t.Fatalf("kind = %q, want sentence", hit.Kind)
	}
}

func TestDetectRepeatedTextCharacterLoop(t *testing.T) {
	text := strings.Repeat("abcdefghijkl", 8)
	hit, ok := detectRepeatedText("content", text, testRepetitionConfig())
	if !ok {
		t.Fatal("expected character repetition hit")
	}
	if hit.Kind != "chars" {
		t.Fatalf("kind = %q, want chars", hit.Kind)
	}
}

func TestDetectRepeatedTextNoFalsePositiveProse(t *testing.T) {
	text := "First we inspect the parser. Then we update the handler. Finally we run the tests. " +
		"The parser rejects invalid input. The handler records the failure. The tests cover the regression."
	if hit, ok := detectRepeatedText("thinking", text, testRepetitionConfig()); ok {
		t.Fatalf("unexpected hit: %+v", hit)
	}
}

func TestDetectRepeatedTextNoFalsePositiveCode(t *testing.T) {
	text := "func main() {\n" +
		"\tfmt.Println(\"hello\")\n" +
		"\tif err != nil {\n" +
		"\t\treturn err\n" +
		"\t}\n" +
		"\tfmt.Println(\"done\")\n" +
		"}\n"
	if hit, ok := detectRepeatedText("content", text, testRepetitionConfig()); ok {
		t.Fatalf("unexpected hit: %+v", hit)
	}
}

func TestDetectRepeatedTextBelowThreshold(t *testing.T) {
	text := strings.Repeat("I should check the file. ", 4)
	if hit, ok := detectRepeatedText("thinking", text, testRepetitionConfig()); ok {
		t.Fatalf("unexpected hit below threshold: %+v", hit)
	}
}

func TestDetectRepeatedTextShortUnitsIgnored(t *testing.T) {
	text := strings.Repeat("ok. ", 40)
	if hit, ok := detectRepeatedText("thinking", text, testRepetitionConfig()); ok {
		t.Fatalf("unexpected hit for short units: %+v", hit)
	}
}

func TestRepetitionDetectorStreaming(t *testing.T) {
	d := newRepetitionDetector("thinking", testRepetitionConfig())
	chunk := "I should check the file. "
	var hit repetitionHit
	detectedAt := -1
	for i := 0; i < 100; i++ {
		h, ok := d.Append(chunk)
		if ok {
			hit = h
			detectedAt = i
			break
		}
	}
	if detectedAt < 0 {
		t.Fatal("expected detection while streaming")
	}
	if detectedAt >= 99 {
		t.Fatalf("detection came too late: chunk %d", detectedAt)
	}
	if hit.Repeats < 5 {
		t.Fatalf("repeats = %d, want >= 5", hit.Repeats)
	}
}

func TestRepetitionDetectorEmptyChunk(t *testing.T) {
	d := newRepetitionDetector("content", testRepetitionConfig())
	if _, ok := d.Append(""); ok {
		t.Fatal("empty chunk must not trigger")
	}
}

func TestRepetitionDetectorWindowCap(t *testing.T) {
	cfg := testRepetitionConfig()
	cfg.WindowChars = 64
	d := newRepetitionDetector("content", cfg)
	d.Append(strings.Repeat("a", 1000))
	if len(d.buf) > cfg.WindowChars {
		t.Fatalf("buffer = %d runes, want <= %d", len(d.buf), cfg.WindowChars)
	}
}

func TestRepetitionDetectorReset(t *testing.T) {
	d := newRepetitionDetector("content", testRepetitionConfig())
	d.Append(strings.Repeat("I should check the file. ", 6))
	d.Reset()
	if len(d.buf) != 0 || d.sinceCheck != 0 {
		t.Fatalf("reset failed: len=%d sinceCheck=%d", len(d.buf), d.sinceCheck)
	}
}

func TestFormatRepetitionHit(t *testing.T) {
	hit := repetitionHit{Source: "thinking", Kind: "words", Unit: "let us check", Repeats: 80}
	got := formatRepetitionHit(hit)
	if !strings.Contains(got, "thinking") || !strings.Contains(got, "80") || !strings.Contains(got, "3-word") {
		t.Fatalf("formatRepetitionHit = %q", got)
	}
}

func TestDetectRepeatedTextMidResponseLineRun(t *testing.T) {
	text := "before the block\n" + strings.Repeat("the same repeated line\n", 7) + "after the block"
	hit, ok := detectRepeatedText("content", text, testRepetitionConfig())
	if !ok {
		t.Fatal("expected mid-response line repetition")
	}
	if hit.Kind != "line" {
		t.Fatalf("kind = %q, want line", hit.Kind)
	}
}

func TestDetectRepeatedTextLoopBeforeConclusion(t *testing.T) {
	text := strings.Repeat("I should check the file. ", 30) + "So the answer is ready."
	hit, ok := detectRepeatedText("thinking", text, testRepetitionConfig())
	if !ok {
		t.Fatal("expected repetition before a trailing conclusion")
	}
	if hit.Repeats < 5 {
		t.Fatalf("repeats = %d, want >= 5", hit.Repeats)
	}
}
