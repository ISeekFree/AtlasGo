package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"iseekfree.com/common/sdk/gomvc/auth"
	"iseekfree.com/common/sdk/gomvc/common"
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
	engine.Use(s.ResponseMiddleware(), s.Recovery(), s.ContextMiddleware())
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
			if !s.authenticate(c, wc, AuthRule{}, true) {
				return
			}
		}
		c.Next()
	}
}

func (s *SDK) Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				if err, ok := recovered.(error); ok {
					writeInternalError(c, err)
				} else {
					c.AbortWithStatusJSON(http.StatusOK, common.Failure(500, "Internal server error"))
				}
			}
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
		wc := s.createContext(c)
		if !s.authenticate(c, wc, rule, true) {
			return
		}
		c.Next()
	}
}

func (s *SDK) authenticate(c *gin.Context, wc *Context, rule AuthRule, required bool) bool {
	if !required {
		return true
	}
	identity, err := s.options.AuthService.Authenticate(c.Request.Context(), s.CreateAuthRequest(wc, c.Request))
	if err != nil {
		writeError(c, err)
		return false
	}
	wc.SetIdentity(identity)
	if len(rule.Domains) > 0 && !contains(rule.Domains, firstNonBlank(identity.Domain, wc.Domain)) {
		writeError(c, common.Unauthorized("Invalid domain visit"))
		return false
	}
	if len(rule.Permissions) > 0 && !auth.IsPermitted(s.options.AuthService, identity, rule.Permissions) {
		writeError(c, common.Unauthorized("No permission"))
		return false
	}
	return true
}

func writeInternalError(c *gin.Context, err error) {
	if clawError, ok := common.AsError(err); ok {
		c.AbortWithStatusJSON(http.StatusOK, common.Failure(clawError.Code, fallback(clawError.Message, "Internal server error")))
		return
	}
	message := err.Error()
	if strings.TrimSpace(message) == "" {
		message = "Internal server error"
	}
	c.AbortWithStatusJSON(http.StatusOK, common.Failure(500, message))
}

func writeError(c *gin.Context, err error) {
	if clawError, ok := common.AsError(err); ok {
		c.AbortWithStatusJSON(http.StatusOK, common.Failure(clawError.Code, fallback(clawError.Message, "Unauthorized")))
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
