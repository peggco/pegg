package agent

import "errors"

var (
	ErrEmptyInput          = errors.New("agent: input must not be empty")
	ErrEmptyResponse       = errors.New("agent: provider returned an empty response")
	ErrMaxIterations       = errors.New("agent: max iterations reached")
	ErrNoProvider          = errors.New("agent: provider is not set")
	ErrToolExecutionFailed = errors.New("agent: tool execution failed")
)
