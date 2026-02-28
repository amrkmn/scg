package app

import "github.com/amrkmn/scg/internal/service"

// Context is the central dependency-injection container passed to all commands.
type Context struct {
	Version  string
	Verbose  bool
	logger   Logger
	Services *Services
}

// Services holds all application service instances.
type Services struct {
	Apps      *service.AppsService
	Buckets   *service.BucketService
	Search    *service.SearchService
	Status    *service.StatusService
	Manifests *service.ManifestService
	Shims     *service.ShimService
	Config    *service.ConfigService
	Cleanup   *service.CleanupService
}

// NewContext constructs a fully wired Context with all services initialised.
func NewContext(version string, verbose bool) *Context {
	ctx := &Context{
		Version: version,
		Verbose: verbose,
	}
	ctx.logger = NewConsoleLogger(verbose)
	ctx.Services = &Services{
		Apps:      service.NewAppsService(ctx),
		Buckets:   service.NewBucketService(ctx),
		Search:    service.NewSearchService(ctx),
		Status:    service.NewStatusService(ctx),
		Manifests: service.NewManifestService(ctx),
		Shims:     service.NewShimService(ctx),
		Config:    service.NewConfigService(ctx),
		Cleanup:   service.NewCleanupService(ctx),
	}
	return ctx
}

// Log delegates to the logger.
func (c *Context) Log(msg string) { c.logger.Log(msg) }

// GetLogger returns the Logger interface (implements service.AppContext).
func (c *Context) GetLogger() service.Logger { return c.logger }

// GetVerbose returns whether verbose mode is enabled.
func (c *Context) GetVerbose() bool { return c.Verbose }

// Logger returns the concrete Logger for use within the app package.
func (c *Context) Logger() Logger { return c.logger }
