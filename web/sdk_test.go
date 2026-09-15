package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ISeekFree/AtlasGo/auth"
	"github.com/ISeekFree/AtlasGo/common"
	"github.com/gin-gonic/gin"
)

func TestSDKWrapsAndAuthenticatesGinRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	codec := auth.NewJWTCodec([]byte("atlas-web-test-jwt-secret-0001-000000"))
	engine := gin.New()
	sdk := New(Options{
		ContextLoaders:    []ContextLoader{testContextLoader(codec)},
		PermissionChecker: testPermissionChecker{},
	})
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

	token, err := codec.Encode(map[string]any{
		"uid":    "u1",
		"domain": "app.demo",
		"perms":  "demo:read",
		"exp":    float64(time.Now().Add(time.Hour).Unix()),
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	authorized := perform(engine, http.MethodGet, "/demo/me", token)
	if authorized.Code != 0 {
		t.Fatalf("authorized code = %d, msg = %s", authorized.Code, authorized.Msg)
	}
	data := authorized.Data.(map[string]any)
	if data["uid"] != "u1" || data["domain"] != "app.demo" {
		t.Fatalf("unexpected response data: %#v", data)
	}
}

func TestContextCustomizerAddsContextAttributes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sdk := New(Options{
		ContextLoaders: []ContextLoader{
			func(wc *Context, request Request) {
				if request.Header("token") != "" {
					wc.UID = "u1"
				}
			},
		},
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
	req.Header.Set("token", "any-token")
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
	data := out.Data.(map[string]any)
	if data["traceId"] != "trace-1" {
		t.Fatalf("unexpected trace attribute: %#v", data)
	}
}

func TestPermissionCheckerFailsClosedWhenAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	codec := auth.NewJWTCodec([]byte("atlas-web-test-jwt-secret-0001-000000"))
	engine := gin.New()
	sdk := New(Options{ContextLoaders: []ContextLoader{testContextLoader(codec)}})
	sdk.Install(engine)
	engine.GET("/demo/me", sdk.RequireAuth(Permissions("demo:read")), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"uid": MustCurrent(c).UID})
	})

	token, err := codec.Encode(map[string]any{
		"uid":   "u1",
		"perms": "demo:read",
		"exp":   float64(time.Now().Add(time.Hour).Unix()),
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	out := perform(engine, http.MethodGet, "/demo/me", token)
	if out.Code != -94 {
		t.Fatalf("permission without checker code = %d, want -94", out.Code)
	}
}

func TestRecoveryReturnsTheUnifiedEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sdk := New()
	sdk.Install(engine)
	engine.GET("/demo/panic", func(c *gin.Context) {
		panic("boom")
	})
	engine.GET("/demo/error", func(c *gin.Context) {
		panic(common.NewErrorCode(-91, "downstream failed"))
	})

	panicResponse := perform(engine, http.MethodGet, "/demo/panic", "")
	if panicResponse.Code != common.SystemErrorCode || panicResponse.Msg != "boom" {
		t.Fatalf("panic envelope = %#v", panicResponse)
	}

	coded := perform(engine, http.MethodGet, "/demo/error", "")
	if coded.Code != -91 || coded.Msg != "downstream failed" {
		t.Fatalf("coded envelope = %#v", coded)
	}
}

func TestErrorResolversCustomiseTheEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sdk := New(Options{
		ErrorResolvers: []ErrorResolver{
			func(_ *gin.Context, failure any) (common.Response[any], bool) {
				if message, ok := failure.(string); ok && message == "boom" {
					return common.Failure(-77, "handled by business"), true
				}
				return common.Response[any]{}, false
			},
		},
	})
	sdk.Install(engine)
	engine.GET("/demo/panic", func(c *gin.Context) {
		panic("boom")
	})

	out := perform(engine, http.MethodGet, "/demo/panic", "")
	if out.Code != -77 || out.Msg != "handled by business" {
		t.Fatalf("resolved envelope = %#v", out)
	}
}

func TestReportedErrorsUseTheUnifiedEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	sdk := New()
	sdk.Install(engine)
	engine.GET("/demo/reported", func(c *gin.Context) {
		_ = c.Error(common.NewErrorCode(-92, "reported failure"))
	})

	out := perform(engine, http.MethodGet, "/demo/reported", "")
	if out.Code != -92 || out.Msg != "reported failure" {
		t.Fatalf("reported envelope = %#v", out)
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

func testContextLoader(codec *auth.JWTCodec) ContextLoader {
	return func(wc *Context, request Request) {
		token := ResolveToken(request, nil, []string{"token", "Authorization", "accessToken"})
		if !token.Present() {
			return
		}
		claims, err := codec.Verify(token.Value)
		if err != nil {
			return
		}
		if uid, ok := claims["uid"].(string); ok {
			wc.UID = uid
		}
		if domain, ok := claims["domain"].(string); ok {
			wc.Domain = domain
		}
		if perms, ok := claims["perms"].(string); ok {
			wc.SetAttribute("perms", perms)
		}
	}
}

type testPermissionChecker struct{}

func (testPermissionChecker) IsPermitted(wc *Context, permissions []string) bool {
	granted, _ := wc.GetAttribute("perms")
	text, _ := granted.(string)
	for _, permission := range permissions {
		if permission != text {
			return false
		}
	}
	return true
}
