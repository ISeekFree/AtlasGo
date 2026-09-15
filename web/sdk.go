package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ISeekFree/AtlasGo/common"
	"github.com/gin-gonic/gin"
)

type SDK struct {
	options Options
}

func New(options ...Options) *SDK {
	if len(options) == 0 {
		return &SDK{options: DefaultOptions()}
	}
	return &SDK{options: mergeOptions(options[0])}
}

func (s *SDK) Install(engine *gin.Engine) {
	engine.Use(s.CorsMiddleware(), s.ResponseMiddleware(), s.Recovery(), s.ContextMiddleware())
}

// CorsMiddleware applies the SDK-wide CORS policy first in the chain. It answers
// a preflight OPTIONS request without touching auth, and decorates every other
// response (including errors) with the configured headers.
func (s *SDK) CorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled(s.options.Enabled) || !enabled(s.options.Cors.Enabled) {
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin != "" && originAllowed(origin, s.options.Cors.AllowedOriginPatterns) {
			cors := s.options.Cors
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", strings.Join(cors.AllowedMethods, ", "))
			c.Header("Access-Control-Allow-Headers", resolveAllowedHeaders(c, cors.AllowedHeaders))
			c.Header("Access-Control-Expose-Headers", strings.Join(cors.ExposedHeaders, ", "))
			if cors.AllowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			if cors.MaxAge > 0 {
				c.Header("Access-Control-Max-Age", strconv.Itoa(int(cors.MaxAge.Seconds())))
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func originAllowed(origin string, patterns []string) bool {
	for _, pattern := range patterns {
		switch {
		case pattern == "*":
			return true
		case strings.HasPrefix(pattern, "*."):
			host := origin
			if idx := strings.Index(origin, "://"); idx >= 0 {
				host = origin[idx+3:]
			}
			if strings.HasSuffix(host, pattern[1:]) {
				return true
			}
		case pattern == origin:
			return true
		}
	}
	return false
}

func resolveAllowedHeaders(c *gin.Context, allowed []string) string {
	for _, header := range allowed {
		if header == "*" {
			if requested := c.GetHeader("Access-Control-Request-Headers"); requested != "" {
				return requested
			}
			return "*"
		}
	}
	return strings.Join(allowed, ", ")
}

func (s *SDK) ContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled(s.options.Enabled) {
			c.Next()
			return
		}
		c.Set(ReqStartKey, time.Now())
		wc := s.createContext(c)
		if c.Request.Method != http.MethodOptions &&
			enabled(s.options.Auth.Enabled) &&
			s.options.Auth.RequiredByDefault &&
			!pathExcluded(c.Request.URL.Path, s.options.Auth.ExcludedPatterns) {
			if !s.authorize(c, wc, AuthRule{}) {
				return
			}
		}
		c.Next()
	}
}

func (s *SDK) Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			c.AbortWithStatusJSON(http.StatusOK, s.resolveErrorResponse(c, recovered))
		}()
		c.Next()
	}
}

type AuthRule struct {
	Domains     []string
	Permissions []string
}

type RuleOption func(*AuthRule)

func Domains(values ...string) RuleOption {
	return func(rule *AuthRule) {
		rule.Domains = append(rule.Domains, values...)
	}
}

func Permissions(values ...string) RuleOption {
	return func(rule *AuthRule) {
		rule.Permissions = append(rule.Permissions, values...)
	}
}

func (s *SDK) RequireAuth(options ...RuleOption) gin.HandlerFunc {
	rule := AuthRule{}
	for _, option := range options {
		option(&rule)
	}
	return func(c *gin.Context) {
		if !enabled(s.options.Enabled) || !enabled(s.options.Auth.Enabled) {
			c.Next()
			return
		}
		if !s.authorize(c, s.createContext(c), rule) {
			return
		}
		c.Next()
	}
}

// authorize enforces the loaded Context: the token loaders have already run,
// so this only checks uid, domain and (optionally) permissions.
func (s *SDK) authorize(c *gin.Context, wc *Context, rule AuthRule) bool {
	if wc == nil || strings.TrimSpace(wc.UID) == "" {
		writeError(c, common.Unauthorized("Unauthorized"))
		return false
	}
	if len(rule.Domains) > 0 && !contains(rule.Domains, wc.Domain) {
		writeError(c, common.Unauthorized("Invalid domain visit"))
		return false
	}
	if len(rule.Permissions) > 0 {
		checker := s.options.PermissionChecker
		if checker == nil || !checker.IsPermitted(wc, rule.Permissions) {
			writeError(c, common.Unauthorized("No permission"))
			return false
		}
	}
	return true
}

// resolveErrorResponse builds the unified envelope for a recovered panic, a
// handler-reported error, or any other failure. Business resolvers run first;
// the SDK then uses the error's own business code and finally falls back to
// common.SystemErrorCode.
func (s *SDK) resolveErrorResponse(c *gin.Context, failure any) common.Response[any] {
	for _, resolver := range s.options.ErrorResolvers {
		if response, ok := resolver(c, failure); ok {
			return response
		}
	}
	var err error
	switch value := failure.(type) {
	case nil:
		return common.Failure(common.SystemErrorCode, "Internal server error")
	case error:
		err = value
	default:
		return common.Failure(common.SystemErrorCode, fallback(fmt.Sprint(value), "Internal server error"))
	}
	if atlasError, ok := common.AsError(err); ok {
		return common.Failure(atlasError.Code, fallback(atlasError.Message, "Internal server error"))
	}
	return common.Failure(common.SystemErrorCode, fallback(err.Error(), "Internal server error"))
}

func writeError(c *gin.Context, err error) {
	if atlasError, ok := common.AsError(err); ok {
		c.AbortWithStatusJSON(http.StatusOK, common.Failure(atlasError.Code, fallback(atlasError.Message, "Unauthorized")))
		return
	}
	message := err.Error()
	if strings.TrimSpace(message) == "" {
		message = "Unauthorized"
	}
	c.AbortWithStatusJSON(http.StatusOK, common.Failure(common.UnauthorizedCode, message))
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func fallback(value string, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func enabled(value *bool) bool {
	return value == nil || *value
}

func pathExcluded(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		switch {
		case strings.HasPrefix(pattern, "*."):
			if strings.HasSuffix(path, pattern[1:]) {
				return true
			}
		case strings.HasSuffix(pattern, "/**"):
			if strings.HasPrefix(path, strings.TrimSuffix(pattern, "**")) {
				return true
			}
		case strings.HasSuffix(pattern, "*"):
			if strings.HasPrefix(path, strings.TrimSuffix(pattern, "*")) {
				return true
			}
		case path == pattern:
			return true
		}
	}
	return false
}
