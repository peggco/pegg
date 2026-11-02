package llm

import "context"

const (
	SessionHeader   = "X-Pegg-Session"
	SessionTemplate = "{{session_id}}"
)

type sessionIDKey struct{}

func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, id)
}

func SessionIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(sessionIDKey{}).(string)
	return id
}

func SessionHeaderFunc(extra []string) func(context.Context) map[string]string {
	return func(ctx context.Context) map[string]string {
		id := SessionIDFrom(ctx)
		if id == "" {
			return nil
		}
		headers := make(map[string]string, 1+len(extra))
		headers[SessionHeader] = id
		for _, name := range extra {
			if name != "" {
				headers[name] = id
			}
		}
		return headers
	}
}
