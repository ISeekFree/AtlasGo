package auth

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func TestCookieStyleServiceAuthenticate(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	token := "_u_=u1;_d_=app.demo;_s_=s1;_exp_=" + strconvFormat(exp) + ";_perms_=demo:read|demo:write"

	identity, err := (CookieStyleService{}).Authenticate(context.Background(), Request{Token: token})
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if identity.UserID != "u1" || identity.Domain != "app.demo" || identity.SessionID != "s1" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
	if !identity.HasAnyPermission([]string{"demo:read"}) {
		t.Fatalf("expected permission to match")
	}
}

func TestCookieStyleServiceRejectsExpiredToken(t *testing.T) {
	_, err := (CookieStyleService{}).Authenticate(context.Background(), Request{Token: "_u_=u1;_exp_=1"})
	if err == nil {
		t.Fatalf("expected expired token error")
	}
}

func strconvFormat(value int64) string {
	return strconv.FormatInt(value, 10)
}
