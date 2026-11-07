package acp

import (
	"context"

	json "github.com/goccy/go-json"

	"github.com/peggco/pegg/internal/core/config"
)

type (
	ClientInfo = Implementation
	AgentInfo  = Implementation
)

type InitializeParams struct {
	Meta               Meta                `json:"_meta,omitempty"`
	ProtocolVersion    int                 `json:"protocolVersion"`
	ClientCapabilities *ClientCapabilities `json:"clientCapabilities,omitempty"`
	ClientInfo         *Implementation     `json:"clientInfo,omitempty"`
}

type ClientCapabilities struct {
	Meta        Meta                     `json:"_meta,omitempty"`
	FS          *FSCapabilities          `json:"fs,omitempty"`
	Terminal    bool                     `json:"terminal,omitempty"`
	Elicitation *ElicitationCapabilities `json:"elicitation,omitempty"`
	Auth        *ClientAuthCapabilities  `json:"auth,omitempty"`
	Session     *ClientSessionCaps       `json:"session,omitempty"`
}

type FSCapabilities struct {
	Meta          Meta `json:"_meta,omitempty"`
	ReadTextFile  bool `json:"readTextFile,omitempty"`
	WriteTextFile bool `json:"writeTextFile,omitempty"`
}

type ElicitationCapabilities struct {
	Meta Meta      `json:"_meta,omitempty"`
	Form *struct{} `json:"form,omitempty"`
	URL  *struct{} `json:"url,omitempty"`
}

type ClientAuthCapabilities struct {
	Meta     Meta `json:"_meta,omitempty"`
	Terminal bool `json:"terminal,omitempty"`
}

type ClientSessionCaps struct {
	Meta          Meta                 `json:"_meta,omitempty"`
	ConfigOptions *ClientConfigOptions `json:"configOptions,omitempty"`
	Compaction    *struct{}            `json:"compaction,omitempty"`
	Notices       *struct{}            `json:"notices,omitempty"`
}

type ClientConfigOptions struct {
	Meta    Meta      `json:"_meta,omitempty"`
	Boolean *struct{} `json:"boolean,omitempty"`
}

type InitializeResult struct {
	Meta              Meta              `json:"_meta,omitempty"`
	ProtocolVersion   int               `json:"protocolVersion"`
	AgentCapabilities AgentCapabilities `json:"agentCapabilities"`
	AgentInfo         *Implementation   `json:"agentInfo,omitempty"`
	AuthMethods       []AuthMethod      `json:"authMethods"`
}

type AgentCapabilities struct {
	Meta               Meta              `json:"_meta,omitempty"`
	LoadSession        bool              `json:"loadSession"`
	PromptCapabilities *PromptCaps       `json:"promptCapabilities,omitempty"`
	MCPCapabilities    *MCPCaps          `json:"mcpCapabilities,omitempty"`
	SessionCaps        *AgentSessionCaps `json:"sessionCapabilities,omitempty"`
	Auth               *AgentAuthCaps    `json:"auth,omitempty"`
}

type PeggCapabilities struct {
	Ask         bool `json:"ask,omitempty"`
	Permissions bool `json:"permissions,omitempty"`
	Models      bool `json:"models,omitempty"`
	Commands    bool `json:"commands,omitempty"`
	Attachments bool `json:"attachments,omitempty"`
	Diffs       bool `json:"diffs,omitempty"`
	Plan        bool `json:"plan,omitempty"`
	History     bool `json:"history,omitempty"`
	Terminals   bool `json:"terminals,omitempty"`
	Elicitation bool `json:"elicitation,omitempty"`
}

type PromptCaps struct {
	Meta            Meta `json:"_meta,omitempty"`
	Image           bool `json:"image,omitempty"`
	Audio           bool `json:"audio,omitempty"`
	EmbeddedContext bool `json:"embeddedContext,omitempty"`
}

type MCPCaps struct {
	Meta Meta `json:"_meta,omitempty"`
	HTTP bool `json:"http,omitempty"`
	SSE  bool `json:"sse,omitempty"`
}

type AgentSessionCaps struct {
	Meta                  Meta              `json:"_meta,omitempty"`
	Delete                *struct{}         `json:"delete,omitempty"`
	Resume                *struct{}         `json:"resume,omitempty"`
	Close                 *struct{}         `json:"close,omitempty"`
	List                  *struct{}         `json:"list,omitempty"`
	AdditionalDirectories *struct{}         `json:"additionalDirectories,omitempty"`
	SystemPrompt          *SystemPromptCaps `json:"systemPrompt,omitempty"`
}

type SystemPromptCaps struct {
	Meta Meta                  `json:"_meta,omitempty"`
	Mode *SystemPromptModeCaps `json:"mode,omitempty"`
}

type SystemPromptModeCaps struct {
	Meta     Meta      `json:"_meta,omitempty"`
	Override *struct{} `json:"override,omitempty"`
}

type AgentAuthCaps struct {
	Meta   Meta      `json:"_meta,omitempty"`
	Logout *struct{} `json:"logout,omitempty"`
}

func (s *Server) handleInitialize(ctx context.Context, params json.RawMessage) (any, error) {
	var req InitializeParams
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "invalid initialize params"}
	}

	if req.ProtocolVersion < 1 {
		return nil, &RPCError{Code: ErrCodeInvalidParams, Message: "unsupported protocol version"}
	}
	s.setClientCapabilities(req.ClientCapabilities)
	s.markInitialized()

	caps := AgentCapabilities{
		LoadSession: true,
		PromptCapabilities: &PromptCaps{
			Image:           true,
			Audio:           true,
			EmbeddedContext: true,
		},
		MCPCapabilities: &MCPCaps{
			HTTP: true,
			SSE:  false,
		},
		SessionCaps: &AgentSessionCaps{
			Delete:                &struct{}{},
			Resume:                &struct{}{},
			Close:                 &struct{}{},
			List:                  &struct{}{},
			AdditionalDirectories: &struct{}{},
			SystemPrompt: &SystemPromptCaps{
				Mode: &SystemPromptModeCaps{Override: &struct{}{}},
			},
		},
		Auth: &AgentAuthCaps{Logout: &struct{}{}},
		Meta: Meta{
			"com.pegg/capabilities": PeggCapabilities{
				Ask:         true,
				Permissions: true,
				Models:      true,
				Commands:    true,
				Attachments: true,
				Diffs:       true,
				Plan:        true,
				History:     true,
				Terminals:   true,
				Elicitation: true,
			},
		},
	}

	return InitializeResult{
		ProtocolVersion:   1,
		AgentCapabilities: caps,
		AgentInfo: &Implementation{
			Name:    "pegg",
			Title:   "Pegg AI Agent",
			Version: config.AppVersion,
		},
		AuthMethods: []AuthMethod{},
	}, nil
}

func (s *Server) handleAuthenticate(ctx context.Context, params json.RawMessage) (any, error) {
	return map[string]any{}, nil
}

func (s *Server) handleLogout(ctx context.Context, params json.RawMessage) (any, error) {
	return map[string]any{}, nil
}
