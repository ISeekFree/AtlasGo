package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"iseekfree.com/common/sdk/gomvc/auth"
)

func TestSDKWrapsAndAuthenticatesGinRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sdk := New()
	sdk.Install(engine)

	engine.GET("/demo/public", func(c *gin.Context) {
		wc := MustCurrent(c)
		c.JSON(http.StatusOK, gin.H{"ip": wc.IP})
	})
	engine.GET("/demo/me", sdk.RequireAuth(Domains("app.demo"), Permissions("demo:read")), func(c *gin.Context) {
		wc := MustCurrent(c)
		c.JSON(http.StatusOK, gin.H{"uid": wc.UID, "domain": wc.Domain})
	})

	public := perform(engine, http.MethodGet, "/demo/public", "")
	if public.Code != 0 {
		t.Fatalf("public code = %d, want 0", public.Code)
	}

	missing := perform(engine, http.MethodGet, "/demo/me", "")
	if missing.Code != -94 {
		t.Fatalf("missing token code = %d, want -94", missing.Code)
	}

	authorized := perform(engine, http.MethodGet, "/demo/me", "_u_=u1;_d_=app.demo;_perms_=demo:read")
	if authorized.Code != 0 {
		t.Fatalf("authorized code = %d, msg = %s", authorized.Code, authorized.Msg)
	}
	data := authorized.Data.(map[string]any)
	if data["uid"] != "u1" || data["domain"] != "app.demo" {
		t.Fatalf("unexpected response data: %#v", data)
	}
}

func TestContextCustomizerAddsAuthRequestAttributes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	authService := &capturingAuthService{}
	engine := gin.New()
	sdk := New(Options{
		AuthService: authService,
		ContextCustomizers: []ContextCustomizer{
			func(wc *Context, c *gin.Context) {
				wc.SetAttribute("demoTraceId", c.GetHeader("x-demo-trace-id"))
			},
		},
	})
	sdk.Install(engine)
	engine.GET("/demo/custom", sdk.RequireAuth(), func(c *gin.Context) {
		traceID, _ := MustCurrent(c).GetAttribute("demoTraceId")
		c.JSON(http.StatusOK, gin.H{"traceId": traceID})
	})

	req := httptest.NewRequest(http.MethodGet, "/demo/custom", nil)
	req.Header.Set("token", "_u_=u1")
	req.Header.Set("x-demo-trace-id", "trace-1")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	var out testResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Code != 0 {
		t.Fatalf("code = %d msg = %s", out.Code, out.Msg)
	}
	if authService.request.Attributes["demoTraceId"] != "trace-1" {
		t.Fatalf("auth attribute demoTraceId = %#v", authService.request.Attributes["demoTraceId"])
	}
}

func perform(engine *gin.Engine, method, path, token string) testResponse {
	req := httptest.NewRequest(method, path, nil)
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

type capturingAuthService struct {
	request auth.Request
}

func (s *capturingAuthService) Authenticate(_ context.Context, request auth.Request) (auth.Identity, error) {
	s.request = request
	return auth.Identity{UserID: "u1", Domain: request.Domain}, nil
}
