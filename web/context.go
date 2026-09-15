package web

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ISeekFree/AtlasGo/auth"
	"github.com/gin-gonic/gin"
)

const (
	GinContextKey = "atlas.web.context"
	ReqStartKey   = "atlas.web.request_started_at"
)

// Request is the transport-neutral view of an incoming call. HTTP, gRPC and
// WebSocket adapt to it, so a single ContextLoader parses the token for every
// transport.
type Request interface {
	Header(name string) string
	Parameter(name string) string
	RemoteIP() string
	Method() string
	Path() string
}

// Context carries the per-request state shared by HTTP, gRPC and WebSocket.
// It is intentionally open for extension: applications either add fields to
// their own wrapper or, more idiomatically in Go, store them through
// SetAttribute. Nothing here is auth-specific; token parsing lives in a
// ContextLoader.
type Context struct {
	UID        string
	IP         string
	OS         string
	OSVersion  string
	Language   string
	Area       string
	Domain     string
	Session    string
	ExpiredAt  *time.Time
	AppVersion string
	AppTag     string
	SimplyArgs string
	Attributes map[string]any
	Request    *http.Request
}

// ContextProvider allocates the per-request Context, letting an application
// pre-seed business attributes before the loaders run.
type ContextProvider func(Request) *Context

// ContextLoader fills token-derived fields (uid, domain, permissions, ...).
// Loaders are transport-neutral and reused by HTTP, gRPC and WebSocket.
type ContextLoader func(*Context, Request)

type ContextCustomizer func(*Context, *gin.Context)

type AuthToken struct {
	Value string
	Admin bool
}

// Present reports whether a non-blank token was found.
func (t AuthToken) Present() bool {
	return strings.TrimSpace(t.Value) != ""
}

func (wc *Context) GetAttribute(key string) (any, bool) {
	if wc == nil || wc.Attributes == nil {
		return nil, false
	}
	value, ok := wc.Attributes[key]
	return value, ok
}

func (wc *Context) Attribute(key string) any {
	value, _ := wc.GetAttribute(key)
	return value
}

func (wc *Context) SetAttribute(key string, value any) {
	if wc == nil || strings.TrimSpace(key) == "" {
		return
	}
	if wc.Attributes == nil {
		wc.Attributes = map[string]any{}
	}
	if value == nil {
		delete(wc.Attributes, key)
		return
	}
	wc.Attributes[key] = value
}

func (wc *Context) ServerName() string {
	if wc == nil || wc.Request == nil {
		return ""
	}
	host := wc.Request.Host
	if host != "" {
		return host
	}
	return wc.Request.URL.Host
}

func (wc *Context) IsIOS() bool {
	info := strings.ToLower(wc.OS + "/" + wc.OSVersion)
	return strings.Contains(info, "iphone") ||
		strings.Contains(info, "ipad") ||
		strings.Contains(info, "ipod") ||
		strings.Contains(info, "ios")
}

func (wc *Context) IsAndroid() bool {
	return strings.Contains(strings.ToLower(wc.OS+"/"+wc.OSVersion), "android")
}

func (wc *Context) IsPC() bool {
	return !wc.IsIOS() && !wc.IsAndroid()
}

func Current(c *gin.Context) (*Context, bool) {
	value, exists := c.Get(GinContextKey)
	if !exists {
		return nil, false
	}
	wc, ok := value.(*Context)
	return wc, ok
}

func MustCurrent(c *gin.Context) *Context {
	if wc, ok := Current(c); ok {
		return wc
	}
	return &Context{Request: c.Request}
}

type contextKey struct{}

func WithContext(ctx context.Context, wc *Context) context.Context {
	return context.WithValue(ctx, contextKey{}, wc)
}

func ContextFromContext(ctx context.Context) (*Context, bool) {
	wc, ok := ctx.Value(contextKey{}).(*Context)
	return wc, ok
}

// HTTPRequest adapts *http.Request onto the transport-neutral Request.
type HTTPRequest struct {
	R *http.Request
}

func (r HTTPRequest) Header(name string) string {
	if r.R == nil {
		return ""
	}
	return r.R.Header.Get(name)
}

func (r HTTPRequest) Parameter(name string) string {
	if r.R == nil {
		return ""
	}
	return r.R.URL.Query().Get(name)
}

func (r HTTPRequest) RemoteIP() string {
	if r.R == nil {
		return ""
	}
	return resolveIP(r.R)
}

func (r HTTPRequest) Method() string {
	if r.R == nil {
		return ""
	}
	return r.R.Method
}

func (r HTTPRequest) Path() string {
	if r.R == nil {
		return ""
	}
	return r.R.URL.Path
}

// ResolveToken is the single token-lookup entry point shared by HTTP, gRPC and
// WebSocket. Admin headers are matched first; an optional "Bearer " prefix is
// always stripped.
func ResolveToken(request Request, adminHeaders, tokenHeaders []string) AuthToken {
	if request == nil {
		return AuthToken{}
	}
	for _, name := range adminHeaders {
		if token := strings.TrimSpace(request.Header(name)); token != "" {
			return AuthToken{Value: auth.StripBearer(token), Admin: true}
		}
	}
	for _, name := range tokenHeaders {
		if token := strings.TrimSpace(request.Header(name)); token != "" {
			return AuthToken{Value: auth.StripBearer(token)}
		}
	}
	return AuthToken{}
}

func (s *SDK) ResolveAuthToken(request Request) AuthToken {
	return ResolveToken(request, s.options.Auth.AdminTokenHeaders, s.options.Auth.TokenHeaders)
}

func (s *SDK) createContext(c *gin.Context) *Context {
	if wc, ok := Current(c); ok {
		return wc
	}
	request := HTTPRequest{R: c.Request}
	wc := &Context{Request: c.Request, Attributes: map[string]any{}}
	if s.options.ContextProvider != nil {
		if provided := s.options.ContextProvider(request); provided != nil {
			wc = provided
		}
	}
	if wc.Attributes == nil {
		wc.Attributes = map[string]any{}
	}
	wc.Request = c.Request
	wc.AppTag = header(c.Request, "appTag")
	wc.OS = header(c.Request, "os")
	wc.OSVersion = header(c.Request, "osv")
	wc.AppVersion = header(c.Request, "av")
	wc.IP = request.RemoteIP()
	wc.SimplyArgs = c.Request.URL.RawQuery
	normalizeLocale(wc, c.Request)
	for _, loader := range s.options.ContextLoaders {
		if loader != nil {
			loader(wc, request)
		}
	}
	for _, customizer := range s.options.ContextCustomizers {
		if customizer != nil {
			customizer(wc, c)
		}
	}

	c.Set(GinContextKey, wc)
	c.Request = c.Request.WithContext(WithContext(c.Request.Context(), wc))
	return wc
}

func normalizeLocale(wc *Context, request *http.Request) {
	tag := request.Header.Get("Accept-Language")
	if tag == "" {
		tag = "en"
	}
	tag = strings.TrimSpace(strings.Split(tag, ",")[0])
	parts := strings.FieldsFunc(tag, func(r rune) bool {
		return r == '-' || r == '_'
	})
	if len(parts) > 0 {
		wc.Language = strings.ToLower(parts[0])
	}
	if len(parts) > 1 {
		wc.Area = strings.ToLower(parts[1])
	}
}

func resolveIP(request *http.Request) string {
	if request == nil {
		return ""
	}
	ip := firstNonBlank(
		header(request, "x-forwarded-for"),
		header(request, "x-real-ip"),
		header(request, "Proxy-Client-IP"),
		header(request, "WL-Proxy-Client-IP"),
		request.RemoteAddr,
	)
	if strings.Contains(ip, ",") {
		ip = strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	host, _, err := net.SplitHostPort(ip)
	if err == nil {
		return host
	}
	return ip
}

func header(request *http.Request, name string) string {
	if request == nil {
		return ""
	}
	return request.Header.Get(name)
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
