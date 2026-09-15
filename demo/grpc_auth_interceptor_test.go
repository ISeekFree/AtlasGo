package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ISeekFree/AtlasGo/auth"
	atlasgrpc "github.com/ISeekFree/AtlasGo/integrations/grpc"
	"github.com/ISeekFree/AtlasGo/web"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestDemoClientAuthInterceptorForwardsCallerToken(t *testing.T) {
	token := demoTestToken(t, map[string]any{"uid": "u1", "domain": "app.demo"})
	request := httptest.NewRequest(http.MethodGet, "/demo/grpc", nil)
	request.Header.Set("token", token)
	ctx := web.WithContext(context.Background(), &web.Context{UID: "u1", Request: request})

	md, ok := metadata.FromOutgoingContext(demoAppendAuthMetadata(ctx, demoClientTokenHeaders...))
	if !ok {
		t.Fatalf("outgoing metadata is missing")
	}
	for _, key := range []string{atlasgrpc.MetadataAuthorization, atlasgrpc.MetadataAccessToken} {
		if got := md.Get(key); len(got) != 1 || got[0] != token {
			t.Fatalf("%s metadata = %#v", key, got)
		}
	}
}

func TestDemoClientAuthInterceptorLeavesContextWithoutCallerToken(t *testing.T) {
	ctx := web.WithContext(context.Background(), &web.Context{Request: httptest.NewRequest(http.MethodGet, "/demo/grpc", nil)})
	if next := demoAppendAuthMetadata(ctx, demoClientTokenHeaders...); next != ctx {
		t.Fatalf("context should be unchanged when the caller sends no token")
	}
}

func TestDemoAuthenticateIncomingResolvesUserToken(t *testing.T) {
	token := demoTestToken(t, map[string]any{"uid": "u1", "domain": "app.demo", "perms": "demo:read"})
	ctx := incomingContext(map[string]string{"accessToken": token})
	next, err := demoAuthenticateIncoming(ctx, demoTestCodec(), "", true)
	if err != nil {
		t.Fatalf("demoAuthenticateIncoming() error = %v", err)
	}
	wc, ok := atlasgrpc.ContextFromContext(next)
	if !ok || wc.UID != "u1" || wc.Domain != "app.demo" {
		t.Fatalf("context = %#v, ok = %v", wc, ok)
	}
	if !containsString(demoGrantedPermissions(wc), "demo:read") {
		t.Fatalf("permissions = %#v", wc.Attribute(demoPermissionsAttribute))
	}
}

func TestDemoAuthenticateIncomingAcceptsInnerToken(t *testing.T) {
	ctx := incomingContext(map[string]string{atlasgrpc.MetadataGrpcToken: "shared-secret"})
	next, err := demoAuthenticateIncoming(ctx, demoTestCodec(), "shared-secret", true)
	if err != nil {
		t.Fatalf("demoAuthenticateIncoming() error = %v", err)
	}
	wc, ok := atlasgrpc.ContextFromContext(next)
	if !ok || wc.UID != "inner" {
		t.Fatalf("context = %#v, ok = %v", wc, ok)
	}
}

func TestDemoAuthenticateIncomingRejectsMissingRequiredToken(t *testing.T) {
	ctx := incomingContext(nil)
	_, err := demoAuthenticateIncoming(ctx, demoTestCodec(), "", true)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("status code = %v, want Unauthenticated (err = %v)", status.Code(err), err)
	}
}

func TestDemoAuthenticateIncomingAllowsAnonymousWhenOptional(t *testing.T) {
	ctx := incomingContext(nil)
	next, err := demoAuthenticateIncoming(ctx, demoTestCodec(), "", false)
	if err != nil {
		t.Fatalf("demoAuthenticateIncoming() error = %v", err)
	}
	if _, ok := atlasgrpc.ContextFromContext(next); ok {
		t.Fatalf("context should be absent for anonymous calls")
	}
}

func demoTestCodec() *auth.JWTCodec {
	return auth.NewJWTCodec([]byte(defaultDemoJWTSecret))
}

func demoTestToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	claims["exp"] = float64(time.Now().Add(time.Hour).Unix())
	token, err := auth.NewJWTCodec([]byte(defaultDemoJWTSecret)).Encode(claims)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	return token
}

func incomingContext(headers map[string]string) context.Context {
	md := metadata.MD{}
	for key, value := range headers {
		md.Set(key, value)
	}
	return metadata.NewIncomingContext(context.Background(), md)
}
