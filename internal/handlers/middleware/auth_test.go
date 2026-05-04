package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedHeaderProvider(t *testing.T) {
	provider := TrustedHeaderProvider{}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Forwarded-User", " alice ")

	userID, err := provider.Principal(request)
	if err != nil {
		t.Fatalf("principal: %v", err)
	}
	if userID != "alice" {
		t.Fatalf("userID = %q, want alice", userID)
	}
}

func TestTrustedHeaderProviderRequiresHeader(t *testing.T) {
	_, err := TrustedHeaderProvider{}.Principal(httptest.NewRequest(http.MethodGet, "/", nil))
	if !errors.Is(err, ErrMissingPrincipal) {
		t.Fatalf("err = %v, want ErrMissingPrincipal", err)
	}
}

func TestNoopProviderReturnsLocalUser(t *testing.T) {
	userID, err := NoopProvider{}.Principal(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("principal: %v", err)
	}
	if userID != LocalUserID {
		t.Fatalf("userID = %q, want %q", userID, LocalUserID)
	}
}

func TestRequirePrincipalStoresPrincipal(t *testing.T) {
	handler := RequirePrincipal(TrustedHeaderProvider{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := Principal(r.Context()); got != "bob" {
			t.Fatalf("principal = %q, want bob", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Forwarded-User", "bob")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestRequirePrincipalAllowsHealthWithoutHeader(t *testing.T) {
	handler := RequirePrincipal(TrustedHeaderProvider{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := Principal(r.Context()); got != LocalUserID {
			t.Fatalf("principal = %q, want %q", got, LocalUserID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}
