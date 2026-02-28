package cmdctx

import (
	"context"

	"github.com/amrkmn/scg/internal/app"
	"github.com/spf13/cobra"
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
func FromCmd(cmd *cobra.Command) *app.Context {
	return FromContext(cmd.Context())
}
