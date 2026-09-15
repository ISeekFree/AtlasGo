package common

import "testing"

func TestNewErrorDefaultsToSystemCode(t *testing.T) {
	err := NewError("boom")
	if err.Code != SystemErrorCode {
		t.Fatalf("NewError code = %d, want %d", err.Code, SystemErrorCode)
	}
	if err.Message != "boom" {
		t.Fatalf("NewError message = %q, want %q", err.Message, "boom")
	}
}

func TestNewErrorCodeKeepsExplicitCode(t *testing.T) {
	err := NewErrorCode(-1001, "custom")
	if err.Code != -1001 {
		t.Fatalf("NewErrorCode code = %d, want -1001", err.Code)
	}
}

func TestWrapErrorDefaultsToSystemCode(t *testing.T) {
	cause := NewError("cause")
	err := WrapError("boom", cause)
	if err.Code != SystemErrorCode {
		t.Fatalf("WrapError code = %d, want %d", err.Code, SystemErrorCode)
	}
	if err.Unwrap() != cause {
		t.Fatalf("WrapError cause = %v, want %v", err.Unwrap(), cause)
	}
}

func TestWrapErrorCodeKeepsExplicitCode(t *testing.T) {
	err := WrapErrorCode(500, "boom", nil)
	if err.Code != 500 {
		t.Fatalf("WrapErrorCode code = %d, want 500", err.Code)
	}
}

func TestUnauthorizedUsesUnauthorizedCode(t *testing.T) {
	err := Unauthorized("")
	if err.Code != UnauthorizedCode {
		t.Fatalf("Unauthorized code = %d, want %d", err.Code, UnauthorizedCode)
	}
	if err.Message != "Unauthorized" {
		t.Fatalf("Unauthorized message = %q, want %q", err.Message, "Unauthorized")
	}
}
