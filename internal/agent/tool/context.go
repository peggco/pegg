package tool

import "context"

type callIDKey struct{}

func WithCallID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, callIDKey{}, id)
}

func CallIDFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(callIDKey{}).(string)
	return id
}
