package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

func TestListPersonasHandler(t *testing.T) {
	response := httptest.NewRecorder()
	ListPersonasHandler(response, httptest.NewRequest(http.MethodGet, "/api/personas", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload []struct {
		ID            string `json:"id"`
		DefaultFormat string `json:"default_format"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode personas: %v", err)
	}
	if len(payload) < 2 {
		t.Fatalf("personas = %d, want at least 2", len(payload))
	}
	if payload[0].ID != "terisuke" || payload[0].DefaultFormat == "" {
		t.Fatalf("unexpected first persona: %#v", payload[0])
	}
}

func TestListFormatsHandler(t *testing.T) {
	response := httptest.NewRecorder()
	ListFormatsHandler(response, httptest.NewRequest(http.MethodGet, "/api/formats", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload []struct {
		ID           string `json:"id"`
		RequiresMeta bool   `json:"requires_meta"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode formats: %v", err)
	}
	foundZenn := false
	for _, format := range payload {
		if format.ID == "zenn_article" && format.RequiresMeta {
			foundZenn = true
		}
	}
	if !foundZenn {
		t.Fatalf("zenn_article format with metadata requirement not found: %#v", payload)
	}
}

func TestGetBriefSessionTemplateHandlerReturnsComposedQuestions(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/brief-sessions/templates?persona_id=cloudia&format_id=zenn_article", nil)

	GetBriefSessionTemplateHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload briefSessionTemplateResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode template: %v", err)
	}
	if payload.PersonaID != personadomain.IDCloudia || payload.OutputFormatID != outputformat.IDZennArticle {
		t.Fatalf("unexpected template identity: %#v", payload)
	}
	if len(payload.Questions) != len(briefdomain.ComposeFixedQuestions(personadomain.IDCloudia, outputformat.IDZennArticle)) {
		t.Fatalf("question count = %d", len(payload.Questions))
	}
	if !hasQuestionJSON(payload.Questions, briefdomain.QuestionIDTargetStack) {
		t.Fatalf("target_stack missing from template: %#v", payload.Questions)
	}
	if !hasQuestionJSON(payload.Questions, briefdomain.QuestionIDCloudiaViewpoint) {
		t.Fatalf("cloudia viewpoint missing from template: %#v", payload.Questions)
	}
}

func TestGetBriefSessionTemplateHandlerDefaultsPersonaFormat(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/brief-sessions/templates?persona_id=cloudia", nil)

	GetBriefSessionTemplateHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload briefSessionTemplateResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode template: %v", err)
	}
	if payload.PersonaID != personadomain.IDCloudia || payload.OutputFormatID != outputformat.IDZennArticle {
		t.Fatalf("unexpected defaulted template identity: %#v", payload)
	}
	if !hasQuestionJSON(payload.Questions, briefdomain.QuestionIDTargetStack) {
		t.Fatalf("default cloudia format did not include technical questions: %#v", payload.Questions)
	}
	if !hasQuestionJSON(payload.Questions, briefdomain.QuestionIDCloudiaViewpoint) {
		t.Fatalf("default cloudia template missing persona extension: %#v", payload.Questions)
	}
}

func TestGetBriefSessionTemplateHandlerValidatesInputs(t *testing.T) {
	tests := []struct {
		name   string
		target string
		status int
		code   string
	}{
		{
			name:   "unknown persona",
			target: "/api/brief-sessions/templates?persona_id=missing",
			status: http.StatusBadRequest,
			code:   "UNKNOWN_PERSONA",
		},
		{
			name:   "unknown output format",
			target: "/api/brief-sessions/templates?persona_id=terisuke&format_id=missing",
			status: http.StatusBadRequest,
			code:   "UNKNOWN_OUTPUT_FORMAT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tt.target, nil)

			GetBriefSessionTemplateHandler(response, request)

			assertErrorResponse(t, response, tt.status, tt.code)
		})
	}
}

func hasQuestionJSON(questions []articleQuestionJSON, id string) bool {
	for _, question := range questions {
		if question.ID == id {
			return true
		}
	}
	return false
}
