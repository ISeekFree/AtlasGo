package web

import (
	"time"

	"github.com/ISeekFree/AtlasGo/common"
	"github.com/gin-gonic/gin"
)

// ErrorResolver maps a recovered panic or a handler-reported failure onto the
// unified {code,msg,data} envelope. Resolvers run in registration order; return
// false to let the next resolver, and finally the SDK default
// (common.SystemErrorCode), decide.
type ErrorResolver func(c *gin.Context, failure any) (common.Response[any], bool)

// PermissionChecker is the business-owned permission check used by
// RequireAuth(Permissions(...)). When no checker is registered, a route that
// demands permissions is rejected (fail closed).
type PermissionChecker interface {
	IsPermitted(context *Context, permissions []string) bool
}

type Options struct {
	Enabled  *bool
	Response ResponseOptions
	Cors     CorsOptions
	Auth     AuthOptions
	// ContextProvider allocates the per-request Context (application
	// subtype/attributes). Optional.
	ContextProvider ContextProvider
	// ContextLoaders parse the token into the Context. Shared by HTTP, gRPC and
	// WebSocket.
	ContextLoaders []ContextLoader
	// ContextCustomizers adjust the HTTP context after loaders run.
	ContextCustomizers []ContextCustomizer
	// PermissionChecker backs RequireAuth(Permissions(...)).
	PermissionChecker PermissionChecker
	// ErrorResolvers customise the envelope returned for recovered panics and
	// handler-reported errors.
	ErrorResolvers []ErrorResolver
}

type ResponseOptions struct {
	Wrap            *bool
	NotWrapPrefixes []string
}

// CorsOptions is the SDK-wide CORS policy installed as the first middleware, so
// preflight requests and error responses are decorated before auth runs. A
// business service no longer registers its own CORS middleware.
type CorsOptions struct {
	Enabled               *bool
	AllowedOriginPatterns []string
	AllowedMethods        []string
	AllowedHeaders        []string
	ExposedHeaders        []string
	AllowCredentials      bool
	MaxAge                time.Duration
}

type AuthOptions struct {
	Enabled           *bool
	RequiredByDefault bool
	// TokenHeaders lists headers carrying an end-user token; they resolve to
	// AuthToken.Admin = false.
	TokenHeaders []string
	// AdminTokenHeaders lists headers carrying an operations/admin token; they
	// are matched first and resolve to AuthToken.Admin = true so a loader can
	// grant elevated privileges.
	AdminTokenHeaders []string
	ExcludedPatterns  []string
}

func Bool(value bool) *bool {
	return &value
}

func DefaultOptions() Options {
	return Options{
		Enabled: Bool(true),
		Response: ResponseOptions{
			Wrap:            Bool(true),
			NotWrapPrefixes: []string{"/actuator", "/metrics", "/healthz"},
		},
		Cors: CorsOptions{
			Enabled:               Bool(true),
			AllowedOriginPatterns: []string{"*"},
			AllowedMethods:        []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:        []string{"*"},
			ExposedHeaders:        []string{"*"},
			AllowCredentials:      false,
			MaxAge:                30 * time.Minute,
		},
		Auth: AuthOptions{
			Enabled:           Bool(true),
			RequiredByDefault: false,
			TokenHeaders:      []string{"token", "Authorization", "accessToken"},
			AdminTokenHeaders: []string{"adminToken"},
			ExcludedPatterns: []string{
				"/favicon.ico",
				"*.css",
				"*.js",
				"*.png",
				"*.jpg",
				"*.jpeg",
			},
		},
	}
}

func mergeOptions(input Options) Options {
	defaults := DefaultOptions()
	if input.Enabled != nil {
		defaults.Enabled = input.Enabled
	}
	if input.Response.Wrap != nil {
		defaults.Response.Wrap = input.Response.Wrap
	}
	if input.Response.NotWrapPrefixes != nil {
		defaults.Response.NotWrapPrefixes = input.Response.NotWrapPrefixes
	}
	if input.Cors.Enabled != nil {
		defaults.Cors.Enabled = input.Cors.Enabled
	}
	if input.Cors.AllowedOriginPatterns != nil {
		defaults.Cors.AllowedOriginPatterns = input.Cors.AllowedOriginPatterns
	}
	if input.Cors.AllowedMethods != nil {
		defaults.Cors.AllowedMethods = input.Cors.AllowedMethods
	}
	if input.Cors.AllowedHeaders != nil {
		defaults.Cors.AllowedHeaders = input.Cors.AllowedHeaders
	}
	if input.Cors.ExposedHeaders != nil {
		defaults.Cors.ExposedHeaders = input.Cors.ExposedHeaders
	}
	defaults.Cors.AllowCredentials = input.Cors.AllowCredentials
	if input.Cors.MaxAge > 0 {
		defaults.Cors.MaxAge = input.Cors.MaxAge
	}
	if input.Auth.Enabled != nil {
		defaults.Auth.Enabled = input.Auth.Enabled
	}
	defaults.Auth.RequiredByDefault = input.Auth.RequiredByDefault
	if input.Auth.TokenHeaders != nil {
		defaults.Auth.TokenHeaders = input.Auth.TokenHeaders
	}
	if input.Auth.AdminTokenHeaders != nil {
		defaults.Auth.AdminTokenHeaders = input.Auth.AdminTokenHeaders
	}
	if input.Auth.ExcludedPatterns != nil {
		defaults.Auth.ExcludedPatterns = input.Auth.ExcludedPatterns
	}
	if input.ContextProvider != nil {
		defaults.ContextProvider = input.ContextProvider
	}
	if input.ContextLoaders != nil {
		defaults.ContextLoaders = input.ContextLoaders
	}
	if input.ContextCustomizers != nil {
		defaults.ContextCustomizers = input.ContextCustomizers
	}
	if input.PermissionChecker != nil {
		defaults.PermissionChecker = input.PermissionChecker
	}
	if input.ErrorResolvers != nil {
		defaults.ErrorResolvers = input.ErrorResolvers
	}
	return defaults
}
