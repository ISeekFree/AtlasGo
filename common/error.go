package common

import (
	"errors"
	"fmt"
)

const UnauthorizedCode = -94

type Error struct {
	Code    int
	Message string
	Cause   error
}

func NewError(code int, message string) *Error {
	return &Error{Code: code, Message: message}
}

func WrapError(code int, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func Unauthorized(message string) *Error {
	if message == "" {
		message = "Unauthorized"
	}
	return NewError(UnauthorizedCode, message)
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
