package sdk

import (
	"github.com/peggco/pegg/internal/agent/agents"
	"github.com/peggco/pegg/internal/agent/middleware"
	"github.com/peggco/pegg/internal/agent/tool"
	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/skill"
)

func (e *Engine) RegisterTool(t Tool) error {
	if err := e.checkOpen(); err != nil {
		return err
	}
	return tool.Default().Register(t)
}

func (e *Engine) UnregisterTool(name string) bool {
	return tool.Default().Unregister(name)
}

func (e *Engine) Tools() []Tool {
	return tool.Default().List()
}

func (e *Engine) RegisterMiddleware(name string, m Middleware) error {
	if err := e.checkOpen(); err != nil {
		return err
	}
	return middleware.Default().Register(name, m)
}

func (e *Engine) UnregisterMiddleware(name string) bool {
	return middleware.Default().Unregister(name)
}

func (e *Engine) RegisterAgent(f AgentFactory) error {
	if err := e.checkOpen(); err != nil {
		return err
	}
	return agents.Register(f)
}

func (e *Engine) RegisterProvider(name string, factory func(cfg LLMConfig) (Provider, error)) error {
	llm.RegisterProvider(name, llm.ProviderFactory(factory))
	return nil
}

func (e *Engine) LoadSkills(dirs ...string) error {
	if err := e.checkOpen(); err != nil {
		return err
	}
	return skill.LoadDirs(dirs...)
}

func (e *Engine) Skills() []*Skill {
	return skill.List()
}

func (e *Engine) RegisterSkill(s *Skill) error {
	return skill.Register(s)
}
