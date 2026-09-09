package auth

import "context"

type Service interface {
	Authenticate(ctx context.Context, request Request) (Identity, error)
}

type PermissionChecker interface {
	IsPermitted(identity Identity, permissions []string) bool
}

func IsPermitted(service Service, identity Identity, permissions []string) bool {
	if len(permissions) == 0 {
		return true
	}
	if checker, ok := service.(PermissionChecker); ok {
		return checker.IsPermitted(identity, permissions)
	}
	return identity.HasAnyPermission(permissions)
}
