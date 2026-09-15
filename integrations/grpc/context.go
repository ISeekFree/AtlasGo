package atlasgrpc

import (
	"context"

	"github.com/ISeekFree/AtlasGo/web"
)

const (
	MetadataAuthorization = "authorization"
	MetadataAccessToken   = "accessToken"
	MetadataGrpcToken     = "grpcToken"
	MetadataForwardedFor  = "x-forwarded-for"
	MetadataRealIP        = "x-real-ip"
)

type grpcContextKey struct{}

// WithContext carries the shared web.Context across a gRPC call. A server
// interceptor loads the context (typically with the same loaders used for
// HTTP/WebSocket) and attaches it here; the service reads it back with
// ContextFromContext.
//
// The gRPC base context is web.Context: use it directly, or embed it in your
// own struct (type NexusContext struct { web.Context; ... }) so HTTP, gRPC and
// WebSocket all share one context type.
func WithContext(ctx context.Context, wc *web.Context) context.Context {
	return context.WithValue(ctx, grpcContextKey{}, wc)
}

// ContextFromContext returns the context attached to a gRPC call, if any.
func ContextFromContext(ctx context.Context) (*web.Context, bool) {
	wc, ok := ctx.Value(grpcContextKey{}).(*web.Context)
	return wc, ok
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	wc, ok := ContextFromContext(ctx)
	if !ok || wc == nil {
		return "", false
	}
	return wc.UID, true
}
