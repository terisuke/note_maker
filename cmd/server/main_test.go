package main

import (
	"net/http"
	"testing"

	"github.com/gorilla/mux"
)

func TestRegisterRoutesIncludesWorkflowReadAPIs(t *testing.T) {
	router := mux.NewRouter()
	registerRoutes(router)

	for _, tt := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/history"},
		{method: http.MethodGet, path: "/api/workflow/artifacts"},
		{method: http.MethodGet, path: "/api/author-style"},
		{method: http.MethodGet, path: "/api/brief-sessions"},
		{method: http.MethodGet, path: "/api/briefs"},
		{method: http.MethodGet, path: "/api/briefs/session-1"},
	} {
		request, err := http.NewRequest(tt.method, tt.path, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		var match mux.RouteMatch
		if !router.Match(request, &match) {
			t.Fatalf("%s %s did not match a route", tt.method, tt.path)
		}
	}
}
