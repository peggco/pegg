package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type HookKind = pkgplugin.HookKind

const (
	HookFilter = pkgplugin.HookFilter
	HookAction = pkgplugin.HookAction
)

const defaultTimeout = 30 * time.Second

var (
	ErrNoHostDispatcher = errors.New("hooks: no host dispatcher available")
	ErrNoHandler        = errors.New("hooks: no handler registered")
)

type Client struct {
	Timeout time.Duration
}

type Outcome struct {
	Value   json.RawMessage   `json:"value"`
	Outputs []json.RawMessage `json:"outputs"`
	Errors  []string          `json:"errors"`
}

func (c *Client) Apply(ctx context.Context, name string, kind HookKind, value any) (Outcome, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Outcome{}, errors.New("hooks: hook name is required")
	}
	if kind == "" {
		kind = HookFilter
	}
	if kind != HookFilter && kind != HookAction {
		return Outcome{}, fmt.Errorf("hooks: hook %q: unknown kind %q", name, kind)
	}

	dispatch := pkgplugin.HostDispatcher()
	if dispatch == nil {
		return Outcome{}, fmt.Errorf("%w: hook %q", ErrNoHostDispatcher, name)
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return Outcome{}, fmt.Errorf("hooks: encode value for hook %q: %w", name, err)
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	req := pkgplugin.HookRequest{
		Name:      name,
		Kind:      kind,
		Value:     payload,
		TimeoutMS: timeout.Milliseconds(),
	}

	type result struct {
		outcome Outcome
		err     error
	}
	done := make(chan result, 1)
	go func() {
		res, err := dispatch(req)
		out := result{err: err}
		if err == nil && res != nil {
			out.outcome = Outcome{Value: res.Value, Outputs: res.Outputs, Errors: res.Errors}
		}
		done <- out
	}()

	select {
	case <-ctx.Done():
		return Outcome{}, fmt.Errorf("hooks: apply %q: %w", name, ctx.Err())
	case r := <-done:
		if r.err != nil {
			if strings.Contains(r.err.Error(), "no handler registered") {
				return Outcome{}, fmt.Errorf("%w: %s", ErrNoHandler, name)
			}
			return Outcome{}, fmt.Errorf("hooks: apply %q: %w", name, r.err)
		}
		return r.outcome, nil
	}
}

func (c *Client) FilterInto(ctx context.Context, name string, value, out any) (Outcome, error) {
	outcome, err := c.Apply(ctx, name, HookFilter, value)
	if err != nil {
		return outcome, err
	}
	if out != nil && len(outcome.Value) > 0 {
		if err := json.Unmarshal(outcome.Value, out); err != nil {
			return outcome, fmt.Errorf("hooks: decode filter %q value: %w", name, err)
		}
	}
	return outcome, nil
}

func (c *Client) Action(ctx context.Context, name string, payload any) (Outcome, error) {
	return c.Apply(ctx, name, HookAction, payload)
}

func (c *Client) Close() error {
	return nil
}
