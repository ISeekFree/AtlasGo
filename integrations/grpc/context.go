package clawgrpc

import (
	"context"

	"github.com/ISeekFree/AtlasGo/auth"
)

const (
	MetadataAuthorization = "authorization"
	MetadataAccessToken   = "accessToken"
	MetadataGrpcToken     = "grpcToken"
	MetadataForwardedFor  = "x-forwarded-for"
	MetadataRealIP        = "x-real-ip"
)

type authContextKey struct{}

type AuthContext struct {
	Identity auth.Identity
	UserID   string
	Domain   string
	IP       string
}

func WithIdentity(ctx context.Context, identity auth.Identity, ip string) context.Context {
	return context.WithValue(ctx, authContextKey{}, AuthContext{
		Identity: identity,
		UserID:   identity.UserID,
		Domain:   identity.Domain,
		IP:       ip,
	})
}

func AuthFromContext(ctx context.Context) (AuthContext, bool) {
	value, ok := ctx.Value(authContextKey{}).(AuthContext)
	return value, ok
}

func IdentityFromContext(ctx context.Context) (auth.Identity, bool) {
	value, ok := AuthFromContext(ctx)
	return value.Identity, ok
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	value, ok := AuthFromContext(ctx)
	return value.UserID, ok
}
