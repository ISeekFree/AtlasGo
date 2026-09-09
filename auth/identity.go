package auth

import "time"

type Identity struct {
	UserID      string         `json:"userId"`
	Domain      string         `json:"domain,omitempty"`
	SessionID   string         `json:"sessionId,omitempty"`
	ExpiresAt   *time.Time     `json:"expiresAt,omitempty"`
	Permissions []string       `json:"permissions,omitempty"`
	Claims      map[string]any `json:"claims,omitempty"`
}

func (i Identity) IsExpired(now time.Time) bool {
	return i.ExpiresAt != nil && !i.ExpiresAt.After(now)
}

func (i Identity) HasAnyPermission(required []string) bool {
	if len(required) == 0 {
		return true
	}
	if len(i.Permissions) == 0 {
		return false
	}
	granted := make(map[string]struct{}, len(i.Permissions))
	for _, permission := range i.Permissions {
		if permission != "" {
			granted[permission] = struct{}{}
		}
	}
	for _, permission := range required {
		if _, ok := granted[permission]; ok {
			return true
		}
	}
	return false
}
