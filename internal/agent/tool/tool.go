package tool

import "context"

type Tool interface {
	Name() string
	Description() string
	Parameters() any
	Execute(ctx context.Context, args string) (string, error)
}

type PermissionAware interface {
	IsPermissionError(err error) bool
}

type Dynamic interface {
	Tool
	IsDynamic() bool
}

type Spec struct {
	name            string
	description     string
	parameters      any
	executeFn       func(ctx context.Context, args string) (string, error)
	permissionError func(error) bool
}

func NewSpec(name, description string, parameters any, fn func(ctx context.Context, args string) (string, error)) *Spec {
	return &Spec{
		name:        name,
		description: description,
		parameters:  parameters,
		executeFn:   fn,
	}
}

func (s *Spec) SetPermissionError(fn func(error) bool) *Spec {
	s.permissionError = fn
	return s
}

func (s *Spec) Name() string {
	return s.name
}

func (s *Spec) Description() string {
	return s.description
}

func (s *Spec) Parameters() any {
	return s.parameters
}

func (s *Spec) Execute(ctx context.Context, args string) (string, error) {
	if s.executeFn == nil {
		return "", ErrNilTool
	}
	return s.executeFn(ctx, args)
}

func (s *Spec) IsPermissionError(err error) bool {
	if s == nil || s.permissionError == nil || err == nil {
		return false
	}
	return s.permissionError(err)
}
