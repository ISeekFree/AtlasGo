package common

import (
	"errors"
	"fmt"
)

const (
	// SystemErrorCode marks a failure that escaped without a business code, for
	// example a recovered panic. It is also the default code of NewError and
	// WrapError.
	SystemErrorCode = -90
	// UnauthorizedCode marks a missing, invalid or expired credential.
	UnauthorizedCode = -94
)

// Error is the unified framework error. HTTP, gRPC and WebSocket all render it
// as the same {code,msg,data} envelope, so a handler only needs to return or
// panic with it.
type Error struct {
	Code    int
	Message string
	Cause   error
}

// NewError builds the unified error with the default code (SystemErrorCode,
// -90). Use NewErrorCode when the caller wants its own business code.
func NewError(message string) *Error {
	return &Error{Code: SystemErrorCode, Message: message}
}

// NewErrorCode builds the unified error with an explicit business code.
func NewErrorCode(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

// WrapError builds the unified error with a cause and the default code (-90).
func WrapError(message string, cause error) *Error {
	return &Error{Code: SystemErrorCode, Message: message, Cause: cause}
}

// WrapErrorCode builds the unified error with an explicit code and a cause.
func WrapErrorCode(code int, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func Unauthorized(message string) *Error {
	if message == "" {
		message = "Unauthorized"
	}
	return NewErrorCode(UnauthorizedCode, message)
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return fmt.Sprintf("%d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%d: %s: %v", e.Code, e.Message, e.Cause)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func AsError(err error) (*Error, bool) {
	var target *Error
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
