package plugin

import (
	"encoding/json"
	"testing"
	"time"

	pkgplugin "github.com/peggco/pegg/pkg/plugin"

	"github.com/peggco/pegg/internal/core/config"
	"github.com/peggco/pegg/internal/core/event"
	"github.com/peggco/pegg/internal/core/logger"
)

type eventPlugin struct {
	pkgplugin.Base
	events chan pkgplugin.Event
	closed bool
}

func (p *eventPlugin) Name() string { return "events" }

func (p *eventPlugin) Capabilities() []string {
	return []string{string(pkgplugin.CapabilityEvents)}
}

func (p *eventPlugin) EventSubscriptions() ([]string, error) {
	return []string{"test.topic"}, nil
}

func (p *eventPlugin) OnEvent(event pkgplugin.Event) error {
	p.events <- event
	return nil
}

func (p *eventPlugin) Close() error {
	p.closed = true
	return nil
}

func TestEventForwarding(t *testing.T) {
	bus := event.New()
	impl := &eventPlugin{events: make(chan pkgplugin.Event, 1)}
	mgr := NewManager(logger.New(logger.LevelError), config.DefaultConfig(), bus, nil, nil, nil, nil, nil, nil)
	instance := &PluginInstance{
		Name:         "events",
		Plugin:       impl,
		Capabilities: impl.Capabilities(),
	}
	mgr.plugins[instance.Name] = instance
	mgr.registerEventSubscriptions(instance)

	bus.Publish("test.topic", map[string]string{"hello": "world"})
	select {
	case got := <-impl.events:
		var payload map[string]string
		if err := json.Unmarshal(got.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if got.Topic != "test.topic" || payload["hello"] != "world" {
			t.Fatalf("unexpected event: %#v %s", got, got.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	mgr.Close()
	if !impl.closed {
		t.Fatal("plugin Close was not called")
	}
}

func TestPermissionDefaultsFromToolSpecs(t *testing.T) {
	mgr := NewManager(logger.New(logger.LevelError), config.DefaultConfig(), nil, nil, nil, nil, nil, nil, nil)
	instance := &PluginInstance{
		Name: "perms",
		Tools: []pkgplugin.ToolSpec{
			{Name: "read_only", ReadOnly: true},
			{Name: "write_something", ReadOnly: false},
		},
	}
	mgr.registerPermissionRules(instance)
	if got := mgr.cfg.Permission.Rules["read_only"]; got != "allow" {
		t.Fatalf("read-only rule: %q", got)
	}
	if got := mgr.cfg.Permission.Rules["write_something"]; got != "semi-judge" {
		t.Fatalf("write rule: %q", got)
	}
}
