package tool

import "errors"

var (
	ErrNilTool        = errors.New("tool: nil tool")
	ErrEmptyName      = errors.New("tool: empty name")
	ErrDuplicate      = errors.New("tool: already registered")
	ErrNotFound       = errors.New("tool: not found")
	ErrDisabled       = errors.New("tool: disabled")
	ErrCommandTimeout = errors.New("command timed out")
)
