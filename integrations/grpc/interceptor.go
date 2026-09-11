package atlasgrpc

import (
	"context"
	"strings"

	"github.com/ISeekFree/AtlasGo/auth"
	"github.com/ISeekFree/AtlasGo/web"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type ServerAuthOptions struct {
	AuthService auth.Service
	Required    bool
	InnerToken  string
}

type ClientAuthOptions struct {
	TokenHeaders      []string
	AdminTokenHeaders []string
}

func UnaryServerAuthInterceptor(input ServerAuthOptions) grpc.UnaryServerInterceptor {
	options := normalizeServerOptions(input)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		nextCtx, err := authenticateIncoming(ctx, options)
		if err != nil {
			return nil, err
		}
		return handler(nextCtx, req)
	}
}

func StreamServerAuthInterceptor(input ServerAuthOptions) grpc.StreamServerInterceptor {
	options := normalizeServerOptions(input)
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		nextCtx, err := authenticateIncoming(stream.Context(), options)
		if err != nil {
			return err
		}
		return handler(srv, &serverStream{ServerStream: stream, ctx: nextCtx})
	}
}

func UnaryClientAuthInterceptor(input ...ClientAuthOptions) grpc.UnaryClientInterceptor {
	options := normalizeClientAuthOptions(input...)
	return func(ctx context.Context, method string, req any, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return invoker(appendAuthMetadata(ctx, options), method, req, reply, cc, opts...)
	}
}

func StreamClientAuthInterceptor(input ...ClientAuthOptions) grpc.StreamClientInterceptor {
	options := normalizeClientAuthOptions(input...)
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return streamer(appendAuthMetadata(ctx, options), desc, cc, method, opts...)
	}
}

type serverStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStream) Context() context.Context {
	return s.ctx
}

func authenticateIncoming(ctx context.Context, options ServerAuthOptions) (context.Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	ip := resolveIP(ctx, md)
	if matchesInnerToken(md.Get(MetadataGrpcToken), options.InnerToken) {
		identity := auth.Identity{
			UserID: "inner",
			Domain: "grpc-inner",
			Claims: map[string]any{"authType": "inner"},
		}
		return WithIdentity(ctx, identity, ip), nil
	}

	token := firstMetadata(md, MetadataAccessToken, MetadataAuthorization)
	if token == "" {
		if options.Required {
			return ctx, status.Error(codes.Unauthenticated, "Missing gRPC token")
		}
		return ctx, nil
	}

	identity, err := options.AuthService.Authenticate(ctx, auth.Request{Token: token, IP: ip})
	if err != nil {
		return ctx, status.Error(codes.Unauthenticated, err.Error())
	}
	return WithIdentity(ctx, identity, ip), nil
}

func appendAuthMetadata(ctx context.Context, options ClientAuthOptions) context.Context {
	wc, ok := web.ContextFromContext(ctx)
	if !ok || wc.Request == nil {
		return ctx
	}
	token := resolveRequestToken(wc.Request.Header.Get, options)
	if strings.TrimSpace(token) == "" {
		return ctx
	}
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	md.Set(MetadataAuthorization, token)
	md.Set(MetadataAccessToken, token)
	return metadata.NewOutgoingContext(ctx, md)
}

func normalizeServerOptions(input ServerAuthOptions) ServerAuthOptions {
	if input.AuthService == nil {
		input.AuthService = auth.CookieStyleService{}
	}
	return input
}

func normalizeClientAuthOptions(input ...ClientAuthOptions) ClientAuthOptions {
	options := ClientAuthOptions{
		TokenHeaders:      []string{"token", "Authorization", "accessToken"},
		AdminTokenHeaders: []string{"adminToken"},
	}
	if len(input) == 0 {
		return options
	}
	if input[0].TokenHeaders != nil {
		options.TokenHeaders = input[0].TokenHeaders
	}
	if input[0].AdminTokenHeaders != nil {
		options.AdminTokenHeaders = input[0].AdminTokenHeaders
	}
	return options
}

func resolveRequestToken(header func(string) string, options ClientAuthOptions) string {
	for _, name := range options.AdminTokenHeaders {
		token := header(name)
		if strings.TrimSpace(token) != "" {
			return token
		}
	}
	for _, name := range options.TokenHeaders {
		token := header(name)
		if strings.TrimSpace(token) != "" {
			return token
		}
	}
	return ""
}

func matchesInnerToken(values []string, expected string) bool {
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

func resolveIP(ctx context.Context, md metadata.MD) string {
	ip := firstMetadata(md, MetadataForwardedFor, MetadataRealIP)
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

func firstMetadata(md metadata.MD, keys ...string) string {
	for _, key := range keys {
		for _, value := range md.Get(key) {
			if strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return ""
}
