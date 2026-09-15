package main

import (
	"context"
	"strings"

	"github.com/ISeekFree/AtlasGo/auth"
	atlasgrpc "github.com/ISeekFree/AtlasGo/integrations/grpc"
	"github.com/ISeekFree/AtlasGo/web"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// demoClientTokenHeaders is the demo's own HTTP token-header contract. AtlasGo
// does not choose these names; the business project does.
var demoClientTokenHeaders = []string{"token", "Authorization", "accessToken"}

// demoUnaryClientAuthInterceptor is the business-layer outbound interceptor used
// by the demo. AtlasGo intentionally does not propagate a Context token on its
// own, because the HTTP header names and the downstream metadata keys are
// business contracts. This interceptor forwards the first non-empty caller
// header as the metadata the demo server reads.
func demoUnaryClientAuthInterceptor(headers ...string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(demoAppendAuthMetadata(ctx, headers...), method, req, reply, cc, opts...)
	}
}

func demoAppendAuthMetadata(ctx context.Context, headers ...string) context.Context {
	wc, ok := web.ContextFromContext(ctx)
	if !ok || wc == nil || wc.Request == nil {
		return ctx
	}
	for _, name := range headers {
		token := strings.TrimSpace(wc.Request.Header.Get(name))
		if token == "" {
			continue
		}
		md, _ := metadata.FromOutgoingContext(ctx)
		md = md.Copy()
		md.Set(atlasgrpc.MetadataAuthorization, token)
		md.Set(atlasgrpc.MetadataAccessToken, token)
		return metadata.NewOutgoingContext(ctx, md)
	}
	return ctx
}

// demoUnaryServerAuthInterceptor is the business-layer inbound gRPC auth used by
// the demo. AtlasGo intentionally no longer ships a server auth interceptor:
// token resolution, inner-token trust, and permission policy stay in the
// business project. This interceptor reuses the same demoContextLoader as HTTP,
// adapting the call through atlasgrpc.MetadataRequest, and attaches the result
// with atlasgrpc.WithContext.
func demoUnaryServerAuthInterceptor(codec *auth.JWTCodec, innerToken string, required bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		next, err := demoAuthenticateIncoming(ctx, codec, innerToken, required)
		if err != nil {
			return nil, err
		}
		return handler(next, req)
	}
}

func demoAuthenticateIncoming(ctx context.Context, codec *auth.JWTCodec, innerToken string, required bool) (context.Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	request := atlasgrpc.MetadataRequest{MD: md, IP: demoResolveIP(ctx, md)}
	wc := &web.Context{IP: request.IP, Attributes: map[string]any{}}
	if demoMatchesInnerToken(md.Get(atlasgrpc.MetadataGrpcToken), innerToken) {
		wc.UID = "inner"
		wc.Domain = "grpc-inner"
		return atlasgrpc.WithContext(ctx, wc), nil
	}

	demoContextLoader(codec)(wc, request)
	if strings.TrimSpace(wc.UID) == "" {
		if required {
			return ctx, status.Error(codes.Unauthenticated, "Missing gRPC token")
		}
		return ctx, nil
	}
	return atlasgrpc.WithContext(ctx, wc), nil
}

func demoMatchesInnerToken(values []string, expected string) bool {
	if strings.TrimSpace(expected) == "" {
		return false
	}
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func demoResolveIP(ctx context.Context, md metadata.MD) string {
	ip := demoFirstMetadata(md, atlasgrpc.MetadataForwardedFor, atlasgrpc.MetadataRealIP)
	if strings.Contains(ip, ",") {
		return strings.TrimSpace(strings.Split(ip, ",")[0])
	}
	if ip != "" {
		return ip
	}
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return p.Addr.String()
	}
	return ""
}

func demoFirstMetadata(md metadata.MD, keys ...string) string {
	for _, key := range keys {
		for _, value := range md.Get(key) {
			if strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return ""
}
