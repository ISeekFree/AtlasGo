package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCorsMiddlewareAnswersPreflightAndEchoesOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	New().Install(engine)
	engine.GET("/demo/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	preflight := httptest.NewRequest(http.MethodOptions, "/demo/ping", nil)
	preflight.Header.Set("Origin", "http://localhost:5173")
	preflight.Header.Set("Access-Control-Request-Method", "GET")
	preflight.Header.Set("Access-Control-Request-Headers", "authorization")
	preflightRecorder := httptest.NewRecorder()
	engine.ServeHTTP(preflightRecorder, preflight)

	if preflightRecorder.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", preflightRecorder.Code, http.StatusNoContent)
	}
	if got := preflightRecorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("preflight allow-origin = %q", got)
	}
	if got := preflightRecorder.Header().Get("Access-Control-Allow-Headers"); got != "authorization" {
		t.Fatalf("preflight allow-headers = %q", got)
	}
	if got := preflightRecorder.Header().Get("Access-Control-Max-Age"); got != "1800" {
		t.Fatalf("preflight max-age = %q", got)
	}

	get := httptest.NewRequest(http.MethodGet, "/demo/ping", nil)
	get.Header.Set("Origin", "http://localhost:5173")
	getRecorder := httptest.NewRecorder()
	engine.ServeHTTP(getRecorder, get)
	if got := getRecorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("get allow-origin = %q", got)
	}
}

func TestCorsMiddlewareCanBeDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	New(Options{Cors: CorsOptions{Enabled: Bool(false)}}).Install(engine)
	engine.GET("/demo/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	request := httptest.NewRequest(http.MethodGet, "/demo/ping", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("allow-origin = %q, want none", got)
	}
}
