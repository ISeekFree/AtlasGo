package web

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ISeekFree/AtlasGo/auth"
	"github.com/gin-gonic/gin"
)

const (
	GinContextKey = "claw.web.context"
	ReqStartKey   = "claw.web.request_started_at"
)

type Context struct {
	UID         string
	IP          string
	OS          string
	OSVersion   string
	Language    string
	Area        string
	Domain      string
	Session     string
	ExpiredAt   *time.Time
	AppVersion  string
	AppTag      string
	PackageName string
	DeviceID    string
	SimplyArgs  string
	Attributes  map[string]any
	Request     *http.Request
}

type ContextCustomizer func(*Context, *gin.Context)

type AuthToken struct {
	Value string
	Admin bool
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

func (wc *Context) ToAuthRequest(token AuthToken) auth.Request {
	attributes := make(map[string]any, len(wc.Attributes)+1)
	for key, value := range wc.Attributes {
		attributes[key] = value
	}
	if wc.DeviceID != "" {
		attributes["deviceId"] = wc.DeviceID
	}
	return auth.Request{
		Token:      token.Value,
		Domain:     wc.Domain,
		AppTag:     wc.AppTag,
		IP:         wc.IP,
		Admin:      token.Admin,
		Attributes: attributes,
	}
}

func (wc *Context) SetIdentity(identity auth.Identity) {
	wc.UID = identity.UserID
	if identity.Domain != "" {
		wc.Domain = identity.Domain
	}
	if identity.SessionID != "" {
		wc.Session = identity.SessionID
	}
	if identity.ExpiresAt != nil {
		wc.ExpiredAt = identity.ExpiresAt
	}
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

func (s *SDK) createContext(c *gin.Context) *Context {
	if wc, ok := Current(c); ok {
		return wc
	}

	wc := &Context{
		Request:     c.Request,
		Attributes:  map[string]any{},
		AppTag:      header(c.Request, "appTag"),
		OS:          header(c.Request, "os"),
		OSVersion:   header(c.Request, "osv"),
		AppVersion:  header(c.Request, "av"),
		PackageName: header(c.Request, "packageName"),
		DeviceID:    firstNonBlank(header(c.Request, "deviceId"), header(c.Request, "udid")),
		IP:          resolveIP(c.Request),
		SimplyArgs:  c.Request.URL.RawQuery,
	}
	normalizeLocale(wc, c.Request)
	s.previewTokenClaims(wc, c.Request)
	for _, customizer := range s.options.ContextCustomizers {
		if customizer != nil {
			customizer(wc, c)
		}
	}

	c.Set(GinContextKey, wc)
	c.Request = c.Request.WithContext(WithContext(c.Request.Context(), wc))
	return wc
}

func (s *SDK) CreateAuthRequest(wc *Context, request *http.Request) auth.Request {
	if wc == nil {
		wc = &Context{Request: request, Attributes: map[string]any{}}
	}
	return wc.ToAuthRequest(s.ResolveAuthToken(request))
}

func (s *SDK) ResolveAuthToken(request *http.Request) AuthToken {
	for _, name := range s.options.Auth.AdminTokenHeaders {
		token := header(request, name)
		if strings.TrimSpace(token) != "" {
			return AuthToken{Value: token, Admin: true}
		}
	}
	for _, name := range s.options.Auth.TokenHeaders {
		token := header(request, name)
		if strings.TrimSpace(token) != "" {
			return AuthToken{Value: token}
		}
	}
	return AuthToken{}
}

func (s *SDK) previewTokenClaims(wc *Context, request *http.Request) {
	token := s.ResolveAuthToken(request)
	if token.Value != "" {
		fillFromToken(wc, token.Value)
	}
}

func fillFromToken(wc *Context, token string) {
	values := auth.ParseCookieStyleToken(auth.StripBearer(token))
	wc.UID = firstNonBlank(values[auth.UID], values["uid"], values["userId"], values["sub"])
	wc.Domain = firstNonBlank(values[auth.Domain], values["domain"])
	wc.Session = firstNonBlank(values[auth.Session], values["session"], values["sid"])
	if expires := firstNonBlank(values[auth.ExpiresAt], values["exp"], values["expiresAt"]); expires != "" {
		if raw, err := strconv.ParseInt(expires, 10, 64); err == nil {
			if raw < 10_000_000_000 {
				raw *= 1000
			}
			parsed := time.UnixMilli(raw)
			wc.ExpiredAt = &parsed
		}
	}
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
