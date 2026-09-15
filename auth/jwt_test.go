package auth

import (
	"strings"
	"testing"
	"time"
)

func TestJWTCodecRoundTripsBusinessClaims(t *testing.T) {
	codec := NewJWTCodec([]byte("atlas-jwt-test-secret-0001-00000000"))
	token, err := codec.Encode(map[string]any{
		"uid": "u1",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	claims, err := codec.Verify(StripBearer("Bearer " + token))
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if claims["uid"] != "u1" {
		t.Fatalf("claims = %#v", claims)
	}
}

func TestJWTCodecRejectsTamperedMalformedAndExpiredTokens(t *testing.T) {
	codec := NewJWTCodec([]byte("atlas-jwt-test-secret-0001-00000000"))
	valid, err := codec.Encode(map[string]any{"uid": "u1", "exp": float64(time.Now().Add(time.Hour).Unix())})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	expired, err := codec.Encode(map[string]any{"uid": "u1", "exp": float64(time.Now().Add(-time.Hour).Unix())})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	for name, token := range map[string]string{
		"tampered":  valid + "x",
		"malformed": "only.two",
		"expired":   expired,
		"empty":     "",
	} {
		if _, err := codec.Verify(token); err == nil {
			t.Fatalf("%s token should be rejected", name)
		}
	}
}

func TestJWTCodecFailsClosedWithoutSecret(t *testing.T) {
	if _, err := NewJWTCodec(nil).Verify("x.y.z"); err == nil {
		t.Fatalf("expected a fail-closed rejection")
	}
}

func TestJWTCodecRejectsAlgorithmConfusion(t *testing.T) {
	secret := []byte("atlas-jwt-test-secret-0001-00000000")
	codec := NewJWTCodec(secret)
	token, err := codec.Encode(map[string]any{"uid": "u1"})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	// A codec pinned to HS512 must not accept an HS256 header.
	other, err := NewJWTCodecWithAlgorithm(secret, AlgorithmHS512)
	if err != nil {
		t.Fatalf("NewJWTCodecWithAlgorithm() error = %v", err)
	}
	if _, err := other.Verify(token); err == nil || !strings.Contains(err.Error(), "algorithm") {
		t.Fatalf("expected an algorithm mismatch, got %v", err)
	}
}
