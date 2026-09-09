package auth

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ISeekFree/AtlasGo/common"
)

const (
	UID         = "_u_"
	Domain      = "_d_"
	Session     = "_s_"
	ExpiresAt   = "_exp_"
	Permissions = "_perms_"
)

type CookieStyleService struct{}

func (CookieStyleService) Authenticate(_ context.Context, request Request) (Identity, error) {
	token := strings.TrimSpace(request.Token)
	if token == "" {
		return Identity{}, common.Unauthorized("Missing token")
	}

	values := ParseCookieStyleToken(StripBearer(token))
	userID := firstNonBlank(values[UID], values["uid"], values["userId"], values["sub"])
	if userID == "" {
		return Identity{}, common.Unauthorized("Invalid token")
	}

	identity := Identity{
		UserID:    userID,
		Domain:    firstNonBlank(values[Domain], values["domain"], request.Domain),
		SessionID: firstNonBlank(values[Session], values["session"], values["sid"]),
		Claims:    make(map[string]any, len(values)),
	}
	for key, value := range values {
		identity.Claims[key] = value
	}
	if expires := firstNonBlank(values[ExpiresAt], values["exp"], values["expiresAt"]); expires != "" {
		if parsed, ok := parseEpoch(expires); ok {
			identity.ExpiresAt = &parsed
		}
	}
	if permissions := firstNonBlank(values[Permissions], values["perms"], values["permissions"]); permissions != "" {
		for _, permission := range strings.FieldsFunc(permissions, func(r rune) bool {
			return r == ',' || r == '|'
		}) {
			permission = strings.TrimSpace(permission)
			if permission != "" {
				identity.Permissions = append(identity.Permissions, permission)
			}
		}
	}
	if identity.IsExpired(time.Now()) {
		return Identity{}, common.Unauthorized("Token expired")
	}
	return identity, nil
}

func (CookieStyleService) IsPermitted(identity Identity, permissions []string) bool {
	return identity.HasAnyPermission(permissions)
}

func ParseCookieStyleToken(token string) map[string]string {
	values := make(map[string]string)
	for _, part := range strings.FieldsFunc(token, func(r rune) bool {
		return r == ';' || r == '&'
	}) {
		index := strings.Index(part, "=")
		if index <= 0 {
			continue
		}
		key := decode(strings.TrimSpace(part[:index]))
		value := decode(strings.TrimSpace(part[index+1:]))
		if key != "" {
			values[key] = value
		}
	}
	return values
}

func StripBearer(token string) string {
	trimmed := strings.TrimSpace(token)
	if len(trimmed) >= 7 && strings.EqualFold(trimmed[:7], "Bearer ") {
		return strings.TrimSpace(trimmed[7:])
	}
	return trimmed
}

func parseEpoch(value string) (time.Time, bool) {
	raw, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	if raw < 10_000_000_000 {
		raw *= 1000
	}
	return time.UnixMilli(raw), true
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func decode(value string) string {
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}
