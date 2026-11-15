package settings

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/peggco/pegg/internal/core/config"
)

func TestMemoryTabFocusReachesStatsAndClear(t *testing.T) {
	s := New(Deps{})
	m := s.memory
	if m.focus != memoryFocusEnabled {
		t.Fatalf("initial focus = %v, want Enabled", m.focus)
	}
	for i := 0; i < 10; i++ {
		if !m.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0)) {
			t.Fatalf("Down not handled at step %d", i)
		}
	}
	if m.focus != memoryFocusClear {
		t.Fatalf("focus = %v, want Clear", m.focus)
	}
	for i := 0; i < 10; i++ {
		if !m.HandleKey(tcell.NewEventKey(tcell.KeyUp, 0, 0)) {
			t.Fatalf("Up not handled at step %d", i)
		}
	}
	if m.focus != memoryFocusEnabled {
		t.Fatalf("focus = %v, want Enabled", m.focus)
	}
}

func TestMemoryTabGateCycling(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := New(Deps{})
	m := s.memory
	m.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	m.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	if m.focus != memoryFocusGate {
		t.Fatalf("focus = %v, want Gate", m.focus)
	}
	start := m.gate
	if !m.HandleKey(tcell.NewEventKey(tcell.KeyRight, 0, 0)) {
		t.Fatal("Right should cycle gate")
	}
	next := m.gate
	if next == start {
		t.Fatal("gate did not change")
	}
	if !m.HandleKey(tcell.NewEventKey(tcell.KeyLeft, 0, 0)) {
		t.Fatal("Left should cycle gate")
	}
	if m.gate != start {
		t.Fatalf("gate = %q, want back to %q", m.gate, start)
	}
	if !m.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)) {
		t.Fatal("Enter should cycle gate")
	}
	if m.gate == start {
		t.Fatal("gate did not change on Enter")
	}
}

func TestMemoryTabStatsRowsReadOnly(t *testing.T) {
	s := New(Deps{})
	m := s.memory
	for i := 0; i < 8; i++ {
		m.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	}
	if m.focus != memoryFocusEntries {
		t.Fatalf("focus = %v, want Entries", m.focus)
	}
	if !m.HandleKey(tcell.NewEventKey(tcell.KeyLeft, 0, 0)) {
		t.Fatal("Left should be consumed on read-only row")
	}
	if !m.HandleKey(tcell.NewEventKey(tcell.KeyRight, 0, 0)) {
		t.Fatal("Right should be consumed on read-only row")
	}
	if !m.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)) {
		t.Fatal("Enter should be consumed on read-only row")
	}
	if m.focus != memoryFocusEntries {
		t.Fatalf("focus changed on read-only row: %v", m.focus)
	}
	m.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	if m.focus != memoryFocusLastEntry {
		t.Fatalf("focus = %v, want LastEntry", m.focus)
	}
	if !m.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)) {
		t.Fatal("Enter should be consumed on read-only row")
	}
	if m.focus != memoryFocusLastEntry {
		t.Fatalf("focus changed on read-only row: %v", m.focus)
	}
}

func TestMemoryTabSavePreservesSkipTools(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.Memory.SkipTools = []string{"websearch"}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	s := New(Deps{Config: cfg})
	m := s.memory
	m.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	m.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	if m.focus != memoryFocusGate {
		t.Fatalf("focus = %v, want Gate", m.focus)
	}
	start := m.gate
	if !m.HandleKey(tcell.NewEventKey(tcell.KeyRight, 0, 0)) {
		t.Fatal("Right should cycle gate")
	}

	got, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Memory.Gate == start {
		t.Fatal("expected memory settings to be toggled and saved")
	}
	if len(got.Memory.SkipTools) != 1 || got.Memory.SkipTools[0] != "websearch" {
		t.Fatalf("skip_tools lost: %+v", got.Memory)
	}
}

func TestMemoryTabSavePersistsObserverAndRelevance(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	thr := 0.7
	cons := false
	cfg := config.DefaultConfig()
	cfg.Memory = &config.MemoryConfig{
		Enabled:       true,
		Observer:      &config.ModelRef{Provider: "obs-prov", Model: "obs-model"},
		Relevance:     &config.ModelRef{Provider: "dec-prov", Model: "dec-model"},
		Gate:          config.GateLLM,
		Threshold:     &thr,
		ContextBudget: 4321,
		MaxResults:    4,
		Consolidate:   &cons,
		SkipTools:     []string{"websearch"},
	}
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	s := New(Deps{Config: cfg})
	m := s.memory
	m.loadIfNeeded()
	if m.provider != "obs-prov" || m.model != "obs-model" {
		t.Fatalf("loaded observer = %q/%q", m.provider, m.model)
	}
	if m.relProvider != "dec-prov" || m.relModel != "dec-model" {
		t.Fatalf("loaded relevance = %q/%q", m.relProvider, m.relModel)
	}

	m.save()
	got, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Memory.Observer == nil || got.Memory.Observer.Provider != "obs-prov" || got.Memory.Observer.Model != "obs-model" {
		t.Fatalf("observer lost: %+v", got.Memory.Observer)
	}
	if got.Memory.Relevance == nil || got.Memory.Relevance.Provider != "dec-prov" || got.Memory.Relevance.Model != "dec-model" {
		t.Fatalf("relevance lost: %+v", got.Memory.Relevance)
	}
	if got.Memory.Provider != "" || got.Memory.Model != "" {
		t.Fatalf("legacy provider/model not cleared: %q/%q", got.Memory.Provider, got.Memory.Model)
	}
	if len(got.Memory.SkipTools) != 1 || got.Memory.SkipTools[0] != "websearch" {
		t.Fatalf("skip_tools lost: %+v", got.Memory)
	}
	if got.Memory.ContextBudget != 4321 || got.Memory.MaxResults != 4 || got.Memory.Gate != config.GateLLM {
		t.Fatalf("memory settings lost: %+v", got.Memory)
	}
	if got.Memory.Threshold == nil || *got.Memory.Threshold != thr {
		t.Fatalf("threshold lost: %+v", got.Memory.Threshold)
	}
	if got.Memory.Consolidate == nil || *got.Memory.Consolidate != cons {
		t.Fatalf("consolidate lost: %+v", got.Memory.Consolidate)
	}

	m.relProvider = "dec-prov-2"
	m.relModel = "dec-model-2"
	m.save()
	got, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Memory.Observer == nil || got.Memory.Observer.Provider != "obs-prov" {
		t.Fatalf("setting relevance cleared observer: %+v", got.Memory.Observer)
	}
	if got.Memory.Relevance == nil || got.Memory.Relevance.Provider != "dec-prov-2" {
		t.Fatalf("relevance not saved: %+v", got.Memory.Relevance)
	}

	m.provider = "obs-prov-2"
	m.model = "obs-model-2"
	m.save()
	got, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Memory.Relevance == nil || got.Memory.Relevance.Provider != "dec-prov-2" {
		t.Fatalf("setting observer cleared relevance: %+v", got.Memory.Relevance)
	}
	if got.Memory.Observer == nil || got.Memory.Observer.Provider != "obs-prov-2" {
		t.Fatalf("observer not saved: %+v", got.Memory.Observer)
	}
}

func TestMemoryTabMigratesLegacyObserver(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.DefaultConfig()
	cfg.Memory.Provider = "legacy-prov"
	cfg.Memory.Model = "legacy-model"
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}

	s := New(Deps{Config: cfg})
	m := s.memory
	m.loadIfNeeded()
	if m.provider != "legacy-prov" || m.model != "legacy-model" {
		t.Fatalf("legacy observer not loaded: %q/%q", m.provider, m.model)
	}

	m.save()
	got, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Memory.Observer == nil || got.Memory.Observer.Provider != "legacy-prov" || got.Memory.Observer.Model != "legacy-model" {
		t.Fatalf("legacy observer not migrated: %+v", got.Memory.Observer)
	}
	if got.Memory.Provider != "" || got.Memory.Model != "" {
		t.Fatalf("legacy provider/model not cleared: %q/%q", got.Memory.Provider, got.Memory.Model)
	}
}
