package web

import "github.com/ISeekFree/AtlasGo/auth"

type Options struct {
	Enabled            *bool
	Response           ResponseOptions
	Auth               AuthOptions
	AuthService        auth.Service
	ContextCustomizers []ContextCustomizer
}

type ResponseOptions struct {
	Wrap            *bool
	NotWrapPrefixes []string
}

type AuthOptions struct {
	Enabled           *bool
	RequiredByDefault bool
	TokenHeaders      []string
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
		AuthService: auth.CookieStyleService{},
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
	if input.AuthService != nil {
		defaults.AuthService = input.AuthService
	}
	if input.ContextCustomizers != nil {
		defaults.ContextCustomizers = input.ContextCustomizers
	}
	return defaults
}
