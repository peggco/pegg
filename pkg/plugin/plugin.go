package plugin

import (
	"encoding/json"
	"errors"
	"net/rpc"

	goplugin "github.com/hashicorp/go-plugin"
)

const ProtocolVersion = 2

type Capability string

const (
	CapabilityTools       Capability = "tools"
	CapabilitySkills      Capability = "skills"
	CapabilityPermissions Capability = "permissions"
	CapabilityCommands    Capability = "commands"
	CapabilityEvents      Capability = "events"
)

var ErrNotSupported = errors.New("plugin: capability not supported")

type Deps struct {
	ConfigJSON []byte
	Options    json.RawMessage
	Workdir    string
	PluginPath string
}

type ToolSpec struct {
	Name        string
	Description string
	Parameters  string
	Bundle      string
	Group       string
	ReadOnly    bool
}

type ToolRequest struct {
	Name      string
	Args      string
	TimeoutMS int64
}

type Attachment struct {
	Type      string
	MediaType string
	FileName  string
	Data      string
	Path      string
}

type ToolResult struct {
	Output      string
	Attachments []Attachment
}

type SkillSpec struct {
	Name         string
	Description  string
	WhenToUse    string
	Instructions string
	ArgumentHint string
	Arguments    []string
	AllowedTools []string
	ToolBundles  []string
	Context      string
	Path         string
}

type FlagSpec struct {
	Name      string
	Shorthand string
	Usage     string
	Default   string
	Bool      bool
}

type CommandSpec struct {
	Use      string
	Short    string
	Long     string
	MinArgs  int
	MaxArgs  int
	Flags    []FlagSpec
	Children []CommandSpec
}

type CommandRequest struct {
	Path  []string
	Args  []string
	Flags map[string]string
}

type CommandResult struct {
	Output string
}

type Event struct {
	Topic   string
	Payload json.RawMessage
}

type Plugin interface {
	Name() string
	Version() string
	Description() string
	Capabilities() []string
	Boot(deps Deps) error

	Tools() ([]ToolSpec, error)
	Skills() ([]SkillSpec, error)
	PermissionRules() (map[string]string, error)
	Commands() ([]CommandSpec, error)
	EventSubscriptions() ([]string, error)

	ExecuteTool(req ToolRequest) (*ToolResult, error)
	RunCommand(req CommandRequest) (*CommandResult, error)
	OnEvent(event Event) error
	Close() error
}

type Base struct{}

func (Base) Name() string                                      { return "" }
func (Base) Version() string                                   { return "0.0.0" }
func (Base) Description() string                               { return "" }
func (Base) Capabilities() []string                            { return nil }
func (Base) Boot(Deps) error                                   { return nil }
func (Base) Tools() ([]ToolSpec, error)                        { return nil, nil }
func (Base) Skills() ([]SkillSpec, error)                      { return nil, nil }
func (Base) PermissionRules() (map[string]string, error)       { return nil, nil }
func (Base) Commands() ([]CommandSpec, error)                  { return nil, nil }
func (Base) EventSubscriptions() ([]string, error)             { return nil, nil }
func (Base) ExecuteTool(ToolRequest) (*ToolResult, error)      { return nil, ErrNotSupported }
func (Base) RunCommand(CommandRequest) (*CommandResult, error) { return nil, ErrNotSupported }
func (Base) OnEvent(Event) error                               { return nil }
func (Base) Close() error                                      { return nil }

func Supports(capabilities []string, wanted Capability) bool {
	if capabilities == nil {
		return true
	}
	for _, capability := range capabilities {
		if capability == string(wanted) {
			return true
		}
	}
	return false
}

var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  ProtocolVersion,
	MagicCookieKey:   "PEGG_PLUGIN",
	MagicCookieValue: "pegg",
}

var PluginMap = map[string]goplugin.Plugin{
	"pegg": &PeggPluginRPC{},
}

func Serve(impl Plugin) {
	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]goplugin.Plugin{
			"pegg": &PeggPluginRPC{Impl: impl},
		},
	})
}

type PeggPluginRPC struct {
	Impl Plugin
}

func (p *PeggPluginRPC) Server(*goplugin.MuxBroker) (interface{}, error) {
	return &PeggPluginRPCServer{impl: p.Impl}, nil
}

func (p *PeggPluginRPC) Client(b *goplugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &PeggPluginRPCClient{client: c}, nil
}

type PeggPluginRPCServer struct {
	impl Plugin
}

func (s *PeggPluginRPCServer) Name(args interface{}, resp *string) error {
	*resp = s.impl.Name()
	return nil
}

func (s *PeggPluginRPCServer) Version(args interface{}, resp *string) error {
	*resp = s.impl.Version()
	return nil
}

func (s *PeggPluginRPCServer) Description(args interface{}, resp *string) error {
	*resp = s.impl.Description()
	return nil
}

func (s *PeggPluginRPCServer) Capabilities(args interface{}, resp *[]string) error {
	*resp = s.impl.Capabilities()
	return nil
}

func (s *PeggPluginRPCServer) Boot(args *Deps, resp *interface{}) error {
	return s.impl.Boot(*args)
}

func (s *PeggPluginRPCServer) Tools(args interface{}, resp *[]ToolSpec) error {
	out, err := s.impl.Tools()
	if err != nil {
		return err
	}
	*resp = out
	return nil
}

func (s *PeggPluginRPCServer) Skills(args interface{}, resp *[]SkillSpec) error {
	out, err := s.impl.Skills()
	if err != nil {
		return err
	}
	*resp = out
	return nil
}

func (s *PeggPluginRPCServer) PermissionRules(args interface{}, resp *map[string]string) error {
	out, err := s.impl.PermissionRules()
	if err != nil {
		return err
	}
	*resp = out
	return nil
}

func (s *PeggPluginRPCServer) Commands(args interface{}, resp *[]CommandSpec) error {
	out, err := s.impl.Commands()
	if err != nil {
		return err
	}
	*resp = out
	return nil
}

func (s *PeggPluginRPCServer) ExecuteTool(args *ToolRequest, resp *ToolResult) error {
	out, err := s.impl.ExecuteTool(*args)
	if err != nil {
		return err
	}
	if out == nil {
		out = &ToolResult{}
	}
	*resp = *out
	return nil
}

func (s *PeggPluginRPCServer) RunCommand(args *CommandRequest, resp *CommandResult) error {
	out, err := s.impl.RunCommand(*args)
	if err != nil {
		return err
	}
	if out == nil {
		out = &CommandResult{}
	}
	*resp = *out
	return nil
}

func (s *PeggPluginRPCServer) EventSubscriptions(args interface{}, resp *[]string) error {
	out, err := s.impl.EventSubscriptions()
	if err != nil {
		return err
	}
	*resp = out
	return nil
}

func (s *PeggPluginRPCServer) OnEvent(args *Event, resp *interface{}) error {
	return s.impl.OnEvent(*args)
}

func (s *PeggPluginRPCServer) Close(args interface{}, resp *interface{}) error {
	return s.impl.Close()
}

type PeggPluginRPCClient struct {
	client *rpc.Client
}

func (c *PeggPluginRPCClient) Name() string {
	var resp string
	if err := c.client.Call("Plugin.Name", new(interface{}), &resp); err != nil {
		return ""
	}
	return resp
}

func (c *PeggPluginRPCClient) Version() string {
	var resp string
	if err := c.client.Call("Plugin.Version", new(interface{}), &resp); err != nil {
		return ""
	}
	return resp
}

func (c *PeggPluginRPCClient) Description() string {
	var resp string
	if err := c.client.Call("Plugin.Description", new(interface{}), &resp); err != nil {
		return ""
	}
	return resp
}

func (c *PeggPluginRPCClient) Capabilities() []string {
	var resp []string
	if err := c.client.Call("Plugin.Capabilities", new(interface{}), &resp); err != nil {
		return nil
	}
	return resp
}

func (c *PeggPluginRPCClient) Boot(deps Deps) error {
	var resp interface{}
	return c.client.Call("Plugin.Boot", &deps, &resp)
}

func (c *PeggPluginRPCClient) Tools() ([]ToolSpec, error) {
	var resp []ToolSpec
	err := c.client.Call("Plugin.Tools", new(interface{}), &resp)
	return resp, err
}

func (c *PeggPluginRPCClient) Skills() ([]SkillSpec, error) {
	var resp []SkillSpec
	err := c.client.Call("Plugin.Skills", new(interface{}), &resp)
	return resp, err
}

func (c *PeggPluginRPCClient) PermissionRules() (map[string]string, error) {
	var resp map[string]string
	err := c.client.Call("Plugin.PermissionRules", new(interface{}), &resp)
	return resp, err
}

func (c *PeggPluginRPCClient) Commands() ([]CommandSpec, error) {
	var resp []CommandSpec
	err := c.client.Call("Plugin.Commands", new(interface{}), &resp)
	return resp, err
}

func (c *PeggPluginRPCClient) ExecuteTool(req ToolRequest) (*ToolResult, error) {
	var resp ToolResult
	if err := c.client.Call("Plugin.ExecuteTool", &req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *PeggPluginRPCClient) RunCommand(req CommandRequest) (*CommandResult, error) {
	var resp CommandResult
	if err := c.client.Call("Plugin.RunCommand", &req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *PeggPluginRPCClient) EventSubscriptions() ([]string, error) {
	var resp []string
	err := c.client.Call("Plugin.EventSubscriptions", new(interface{}), &resp)
	return resp, err
}

func (c *PeggPluginRPCClient) OnEvent(event Event) error {
	var resp interface{}
	return c.client.Call("Plugin.OnEvent", &event, &resp)
}

func (c *PeggPluginRPCClient) Close() error {
	var resp interface{}
	return c.client.Call("Plugin.Close", new(interface{}), &resp)
}
