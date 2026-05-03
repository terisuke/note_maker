package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	"github.com/teradakousuke/note_maker/internal/infrastructure/repository/memory"
)

func TestWorkflowHistoryHandlersReturnSavedArtifacts(t *testing.T) {
	style := setupWorkflowStyle(t)
	completed := sessionWithFixedAnswers(t, "session-completed-history", style.Profile.ID)
	completed.MarkDeepDiveSkipped()
	brief, err := completed.Complete()
	if err != nil {
		t.Fatalf("complete session: %v", err)
	}
	if err := workflowStore.SaveSession(completed); err != nil {
		t.Fatalf("save completed session: %v", err)
	}
	if err := workflowStore.SaveBrief(completed.ID, brief); err != nil {
		t.Fatalf("save brief: %v", err)
	}
	open, err := briefdomain.NewArticleBriefSession("session-open-history", style.Profile.ID)
	if err != nil {
		t.Fatalf("new open session: %v", err)
	}
	if err := workflowStore.SaveSession(open); err != nil {
		t.Fatalf("save open session: %v", err)
	}

	t.Run("author styles", func(t *testing.T) {
		response := httptest.NewRecorder()
		ListAuthorStylesHandler(response, httptest.NewRequest(http.MethodGet, "/api/author-style", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
		var payload authorStyleListResponse
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(payload.StyleGuides) != 1 || payload.StyleGuides[0].ProfileID != style.Profile.ID || payload.StyleGuides[0].GuideMarkdown == "" {
			t.Fatalf("unexpected style guide list: %#v", payload)
		}
	})

	t.Run("sessions", func(t *testing.T) {
		response := httptest.NewRecorder()
		ListBriefSessionsHandler(response, httptest.NewRequest(http.MethodGet, "/api/brief-sessions", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
		var payload briefSessionListResponse
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(payload.Sessions) != 2 {
			t.Fatalf("sessions = %d, want 2: %#v", len(payload.Sessions), payload)
		}
		var completedSummary briefSessionSummaryResponse
		for _, item := range payload.Sessions {
			if item.SessionID == completed.ID {
				completedSummary = item
			}
		}
		if !completedSummary.Completed || !completedSummary.BriefAvailable || completedSummary.Title != brief.Theme {
			t.Fatalf("unexpected completed session summary: %#v", completedSummary)
		}
	})

	t.Run("briefs", func(t *testing.T) {
		response := httptest.NewRecorder()
		ListBriefArtifactsHandler(response, httptest.NewRequest(http.MethodGet, "/api/briefs", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
		var payload briefArtifactListResponse
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(payload.Briefs) != 1 || payload.Briefs[0].SessionID != completed.ID || payload.Briefs[0].Brief.Theme != brief.Theme {
			t.Fatalf("unexpected brief list: %#v", payload)
		}
	})

	t.Run("workflow artifacts", func(t *testing.T) {
		response := httptest.NewRecorder()
		ListWorkflowArtifactsHandler(response, httptest.NewRequest(http.MethodGet, "/api/workflow/artifacts", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
		}
		var payload workflowArtifactsResponse
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(payload.StyleGuides) != 1 || len(payload.Sessions) != 2 || len(payload.Briefs) != 1 {
			t.Fatalf("unexpected workflow artifacts: %#v", payload)
		}
	})
}

func TestGetBriefArtifactHandler(t *testing.T) {
	style := setupWorkflowStyle(t)
	session := sessionWithFixedAnswers(t, "session-brief-detail", style.Profile.ID)
	session.MarkDeepDiveSkipped()
	brief, err := session.Complete()
	if err != nil {
		t.Fatalf("complete session: %v", err)
	}
	if err := workflowStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := workflowStore.SaveBrief(session.ID, brief); err != nil {
		t.Fatalf("save brief: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/briefs/session-brief-detail", nil)
	request = mux.SetURLVars(request, map[string]string{"id": session.ID})
	response := httptest.NewRecorder()

	GetBriefArtifactHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload briefArtifactResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.SessionID != session.ID || payload.Title != brief.Theme || payload.AnswerCount != len(session.Answers) {
		t.Fatalf("unexpected brief artifact: %#v", payload)
	}
}

func TestGetBriefArtifactHandlerNotFound(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	request := httptest.NewRequest(http.MethodGet, "/api/briefs/missing", nil)
	request = mux.SetURLVars(request, map[string]string{"id": "missing"})
	response := httptest.NewRecorder()

	GetBriefArtifactHandler(response, request)

	assertErrorResponse(t, response, http.StatusNotFound, "BRIEF_NOT_FOUND")
}
