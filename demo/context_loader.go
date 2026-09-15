package main

import (
	"strings"
	"time"

	"github.com/ISeekFree/AtlasGo/auth"
	"github.com/ISeekFree/AtlasGo/web"
)

// defaultDemoJWTSecret keeps the demo runnable; production overrides it with
// ATLAS_DEMO_JWT_SECRET.
const defaultDemoJWTSecret = "atlas-demo-dev-only-jwt-secret-change-me-0001"

// demoPermissionsAttribute is where the demo stashes the claim-derived
// permission set on the shared Context.
const demoPermissionsAttribute = "demoPermissions"

// demoAdminAttribute is where the demo stashes the claim-derived admin flag.
const demoAdminAttribute = "demoAdmin"

// demoAdminTokenHeaders is the demo's own admin-header contract.
var demoAdminTokenHeaders = []string{"adminToken"}

// demoContextLoader is the demo's token contract, shared by HTTP, gRPC and
// WebSocket. The names uid/domain/session/perms belong to the demo; AtlasGo
// verifies the JWT and never reads them.
func demoContextLoader(codec *auth.JWTCodec) web.ContextLoader {
	return func(wc *web.Context, request web.Request) {
		if codec == nil {
			return
		}
		token := web.ResolveToken(request, demoAdminTokenHeaders, demoClientTokenHeaders)
		if !token.Present() {
			return
		}
		claims, err := codec.Verify(token.Value)
		if err != nil {
			return
		}
		uid := claimText(claims["uid"])
		if uid == "" {
			return
		}
		wc.UID = uid
		wc.SetAttribute(demoAdminAttribute, token.Admin || claimBool(claims["admin"]))
		if domain := claimText(claims["domain"]); domain != "" {
			wc.Domain = domain
		}
		if session := claimText(claims["session"]); session != "" {
			wc.Session = session
		}
		if exp, ok := claims["exp"].(float64); ok {
			expires := time.Unix(int64(exp), 0)
			wc.ExpiredAt = &expires
		}
		wc.SetAttribute(demoPermissionsAttribute, claimPermissions(claims["perms"]))
	}
}

// demoPermissionChecker is the business-owned check behind
// RequireAuth(Permissions(...)).
type demoPermissionChecker struct{}

func (demoPermissionChecker) IsPermitted(wc *web.Context, permissions []string) bool {
	granted := demoGrantedPermissions(wc)
	for _, permission := range permissions {
		if !containsString(granted, permission) {
			return false
		}
	}
	return true
}

// demoGrantedPermissions reads the permission set the loader stored on the
// context.
func demoGrantedPermissions(wc *web.Context) []string {
	if wc == nil {
		return nil
	}
	value, ok := wc.GetAttribute(demoPermissionsAttribute)
	if !ok {
		return nil
	}
	permissions, _ := value.([]string)
	return permissions
}

func claimPermissions(value any) []string {
	text := claimText(value)
	if text == "" {
		return nil
	}
	permissions := []string{}
	for _, permission := range strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == '|' }) {
		if trimmed := strings.TrimSpace(permission); trimmed != "" {
			permissions = append(permissions, trimmed)
		}
	}
	return permissions
}

func claimText(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func claimBool(value any) bool {
	if flag, ok := value.(bool); ok {
		return flag
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
