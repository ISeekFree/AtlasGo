package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDemoAppValidatesWebAndGRPCAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine, cleanup, err := newDemoApp(context.Background(), Config{})
	if err != nil {
		t.Fatalf("newDemoApp() error = %v", err)
	}
	defer cleanup(context.Background())

	public := request(engine, "/demo/public", "")
	if public.Code != 0 {
		t.Fatalf("public code = %d", public.Code)
	}

	missing := request(engine, "/demo/me", "")
	if missing.Code != -94 {
		t.Fatalf("missing token code = %d, want -94", missing.Code)
	}

	token := "_u_=u1;_d_=app.demo;_perms_=demo:read"
	me := request(engine, "/demo/me", token)
	if me.Code != 0 {
		t.Fatalf("me code = %d msg = %s", me.Code, me.Msg)
	}
	meData := me.Data.(map[string]any)
	if meData["uid"] != "u1" || meData["domain"] != "app.demo" {
		t.Fatalf("unexpected /demo/me data: %#v", meData)
	}

	grpcResp := request(engine, "/demo/grpc", token)
	if grpcResp.Code != 0 {
		t.Fatalf("grpc code = %d msg = %s", grpcResp.Code, grpcResp.Msg)
	}
	if grpcResp.Data != "echo:u1:ping" {
		t.Fatalf("grpc data = %#v, want echo:u1:ping", grpcResp.Data)
	}

	grpcEcho := request(engine, "/demo/grpc/echo?message=hello", token)
	if grpcEcho.Code != 0 {
		t.Fatalf("grpc echo code = %d msg = %s", grpcEcho.Code, grpcEcho.Msg)
	}
	echoData := grpcEcho.Data.(map[string]any)
	if echoData["message"] != "echo:u1:hello" || echoData["userId"] != "u1" {
		t.Fatalf("unexpected gRPC Echo data: %#v", echoData)
	}

	grpcMe := request(engine, "/demo/grpc/me", token)
	if grpcMe.Code != 0 {
		t.Fatalf("grpc me code = %d msg = %s", grpcMe.Code, grpcMe.Msg)
	}
	grpcMeData := grpcMe.Data.(map[string]any)
	if grpcMeData["userId"] != "u1" || grpcMeData["domain"] != "app.demo" {
		t.Fatalf("unexpected gRPC CurrentUser data: %#v", grpcMeData)
	}
	permissions := grpcMeData["permissions"].([]any)
	if len(permissions) != 1 || permissions[0] != "demo:read" {
		t.Fatalf("unexpected gRPC CurrentUser permissions: %#v", permissions)
	}
}

func request(engine *gin.Engine, path string, token string) testResponse {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("token", token)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	var out testResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		panic(err)
	}
	return out
}

type testResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}
