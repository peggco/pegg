package plugin

import (
	"encoding/json"
	"net"
	"net/rpc"
	"testing"
)

type testPlugin struct {
	Base
	events []Event
	closed bool
}

func (p *testPlugin) Name() string        { return "test" }
func (p *testPlugin) Version() string     { return "1.2.3" }
func (p *testPlugin) Description() string { return "test plugin" }

func (p *testPlugin) Capabilities() []string {
	return []string{"tools", "skills", "permissions", "commands", "events"}
}

func (p *testPlugin) Tools() ([]ToolSpec, error) {
	return []ToolSpec{{Name: "echo", Bundle: "test", Group: "Test", ReadOnly: true, Parameters: `{"type":"object"}`}}, nil
}

func (p *testPlugin) Skills() ([]SkillSpec, error) {
	return []SkillSpec{{Name: "test-skill", Instructions: "do it"}}, nil
}

func (p *testPlugin) PermissionRules() (map[string]string, error) {
	return map[string]string{"echo": "allow"}, nil
}

func (p *testPlugin) Commands() ([]CommandSpec, error) {
	return []CommandSpec{{Use: "test", Short: "test"}}, nil
}

func (p *testPlugin) EventSubscriptions() ([]string, error) {
	return []string{"test.topic"}, nil
}

func (p *testPlugin) ExecuteTool(req ToolRequest) (*ToolResult, error) {
	return &ToolResult{Output: req.Name + ":" + req.Args}, nil
}

func (p *testPlugin) OnEvent(event Event) error {
	p.events = append(p.events, event)
	return nil
}

func (p *testPlugin) Close() error {
	p.closed = true
	return nil
}

func TestRPCProtocol(t *testing.T) {
	impl := &testPlugin{}
	serverConn, clientConn := net.Pipe()
	server := rpc.NewServer()
	if err := server.RegisterName("Plugin", &PeggPluginRPCServer{impl: impl}); err != nil {
		t.Fatal(err)
	}
	go server.ServeConn(serverConn)
	client := rpc.NewClient(clientConn)
	defer client.Close()
	plugin := &PeggPluginRPCClient{client: client}

	if plugin.Name() != "test" || plugin.Version() != "1.2.3" || plugin.Description() != "test plugin" {
		t.Fatalf("metadata mismatch")
	}
	if len(plugin.Capabilities()) != 5 {
		t.Fatalf("capabilities: %v", plugin.Capabilities())
	}
	tools, err := plugin.Tools()
	if err != nil || len(tools) != 1 || tools[0].Name != "echo" || !tools[0].ReadOnly {
		t.Fatalf("tools: %#v %v", tools, err)
	}
	skills, err := plugin.Skills()
	if err != nil || len(skills) != 1 || skills[0].Name != "test-skill" {
		t.Fatalf("skills: %#v %v", skills, err)
	}
	rules, err := plugin.PermissionRules()
	if err != nil || rules["echo"] != "allow" {
		t.Fatalf("rules: %#v %v", rules, err)
	}
	commands, err := plugin.Commands()
	if err != nil || len(commands) != 1 || commands[0].Use != "test" {
		t.Fatalf("commands: %#v %v", commands, err)
	}
	subs, err := plugin.EventSubscriptions()
	if err != nil || len(subs) != 1 || subs[0] != "test.topic" {
		t.Fatalf("subscriptions: %#v %v", subs, err)
	}
	result, err := plugin.ExecuteTool(ToolRequest{Name: "echo", Args: `{"x":1}`})
	if err != nil || result.Output != `echo:{"x":1}` {
		t.Fatalf("tool result: %#v %v", result, err)
	}
	payload, _ := json.Marshal(map[string]string{"x": "y"})
	if err := plugin.OnEvent(Event{Topic: "test.topic", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	if len(impl.events) != 1 || impl.events[0].Topic != "test.topic" {
		t.Fatalf("events: %#v", impl.events)
	}
	if err := plugin.Close(); err != nil || !impl.closed {
		t.Fatalf("close: %v %t", err, impl.closed)
	}
}

func TestBaseDefaults(t *testing.T) {
	var base Base
	if !Supports(base.Capabilities(), CapabilityTools) {
		t.Fatal("nil capabilities should support all")
	}
	if Supports([]string{"skills"}, CapabilityTools) {
		t.Fatal("explicit capabilities should restrict")
	}
	if _, err := base.ExecuteTool(ToolRequest{}); err != ErrNotSupported {
		t.Fatalf("unexpected tool error: %v", err)
	}
	if _, err := base.RunCommand(CommandRequest{}); err != ErrNotSupported {
		t.Fatalf("unexpected command error: %v", err)
	}
	if err := base.Close(); err != nil {
		t.Fatal(err)
	}
}
