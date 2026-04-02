package cmdctx

import (
	"context"

	"github.com/spf13/cobra"
	"go.noz.one/scg/internal/app"
)

type contextKey struct{}

// Inject stores an app.Context into a context.Context.
func Inject(parent context.Context, appCtx *app.Context) context.Context {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithValue(parent, contextKey{}, appCtx)
}

// FromContext retrieves the app.Context from a context.Context.
func FromContext(ctx context.Context) *app.Context {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(contextKey{}).(*app.Context)
	return v
}

// FromCmd retrieves the app.Context from a cobra command's context.
// Returns nil if no context is found.
func FromCmd(cmd *cobra.Command) *app.Context {
	return FromContext(cmd.Context())
}

// MustFromCmd retrieves the app.Context from a cobra command's context.
// It panics if no context is found, which indicates a programming error
// (the root command's PersistentPreRunE should always inject context).
// Use this in commands where context is required.
func MustFromCmd(cmd *cobra.Command) *app.Context {
	ctx := FromCmd(cmd)
	if ctx == nil {
		panic("context unavailable: this is a programming error - context should be injected by root command")
	}
	return ctx
}
