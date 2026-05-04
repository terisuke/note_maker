package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

const LocalUserID = "local"

var (
	ErrMissingPrincipal = errors.New("missing authenticated principal")
	ErrNotImplemented   = errors.New("auth provider is not implemented")
)

type contextKey string

const principalContextKey contextKey = "note-maker-principal"

// AuthProvider resolves the authenticated user for one request.
type AuthProvider interface {
	Principal(r *http.Request) (string, error)
}

// NoopProvider preserves single-user behavior.
type NoopProvider struct{}

func (NoopProvider) Principal(*http.Request) (string, error) {
	return LocalUserID, nil
}

// TrustedHeaderProvider trusts a reverse proxy to set an authenticated user.
type TrustedHeaderProvider struct {
	HeaderName string
}

func (p TrustedHeaderProvider) Principal(r *http.Request) (string, error) {
	header := strings.TrimSpace(p.HeaderName)
	if header == "" {
		header = "X-Forwarded-User"
	}
	userID := strings.TrimSpace(r.Header.Get(header))
	if userID == "" {
		return "", ErrMissingPrincipal
	}
	return userID, nil
}

type OIDCProvider struct{}

func (OIDCProvider) Principal(*http.Request) (string, error) {
	return "", ErrNotImplemented
}

type MagicLinkProvider struct{}

func (MagicLinkProvider) Principal(*http.Request) (string, error) {
	return "", ErrNotImplemented
}

func RequirePrincipal(provider AuthProvider) func(http.Handler) http.Handler {
	if provider == nil {
		provider = NoopProvider{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublicHealthPath(r.URL.Path) {
				next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), LocalUserID)))
				return
			}
			userID, err := provider.Principal(r)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), userID)))
		})
	}
}

func isPublicHealthPath(path string) bool {
	return path == "/healthz"
}

func WithPrincipal(ctx context.Context, userID string) context.Context {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = LocalUserID
	}
	return context.WithValue(ctx, principalContextKey, userID)
}

func Principal(ctx context.Context) string {
	if ctx == nil {
		return LocalUserID
	}
	userID, _ := ctx.Value(principalContextKey).(string)
	if strings.TrimSpace(userID) == "" {
		return LocalUserID
	}
	return strings.TrimSpace(userID)
}
