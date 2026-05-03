package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
	"github.com/teradakousuke/note_maker/internal/infrastructure/repository/memory"
)

func TestCreatePersonaHandlerStoresCustomPersonaAndListKeepsBuiltIns(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	body := `{"id":"custom_writer","display_name":"Custom Writer","default_format":"note_article"}`
	response := httptest.NewRecorder()

	CreatePersonaHandler(response, httptest.NewRequest(http.MethodPost, "/api/personas", bytes.NewBufferString(body)))

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var created personadomain.Persona
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID != "custom_writer" || created.DefaultFormat != outputformat.IDNoteArticle {
		t.Fatalf("unexpected created persona: %#v", created)
	}
	if created.Description == "" || created.VoiceNotes.Tone == "" {
		t.Fatalf("expected defaulted optional persona fields: %#v", created)
	}

	listResponse := httptest.NewRecorder()
	ListPersonasHandler(listResponse, httptest.NewRequest(http.MethodGet, "/api/personas", nil))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	var personas []personadomain.Persona
	if err := json.NewDecoder(listResponse.Body).Decode(&personas); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(personas) < 3 || personas[0].ID != personadomain.IDTerisuke || personas[1].ID != personadomain.IDCloudia {
		t.Fatalf("built-in personas were not preserved first: %#v", personas)
	}
	if personas[len(personas)-1].ID != "custom_writer" {
		t.Fatalf("custom persona missing from list: %#v", personas)
	}
	if _, ok := workflowStore.GetPersona("custom_writer"); !ok {
		t.Fatal("custom persona was not saved")
	}
}

func TestUpdatePersonaHandlerPreservesIDAndStoresFields(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	if err := workflowStore.SavePersona(personadomain.Persona{
		ID:            "custom_writer",
		DisplayName:   "Custom Writer",
		Description:   "Old description",
		DefaultFormat: outputformat.IDNoteArticle,
		VoiceNotes: personadomain.VoiceNotes{
			Tone: "Old tone.",
		},
	}); err != nil {
		t.Fatalf("save persona: %v", err)
	}

	request := httptest.NewRequest(http.MethodPatch, "/api/personas/custom_writer", bytes.NewBufferString(`{
		"display_name":"Updated Writer",
		"default_format":"zenn_article",
		"voice_notes":{"tone":"Sharper and more technical.","first_person":["私"]}
	}`))
	request = mux.SetURLVars(request, map[string]string{"id": "custom_writer"})
	response := httptest.NewRecorder()

	UpdatePersonaHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var updated personadomain.Persona
	if err := json.NewDecoder(response.Body).Decode(&updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.ID != "custom_writer" || updated.DisplayName != "Updated Writer" || updated.DefaultFormat != outputformat.IDZennArticle {
		t.Fatalf("unexpected updated persona: %#v", updated)
	}
	restored, ok := workflowStore.GetPersona("custom_writer")
	if !ok || restored.VoiceNotes.Tone != "Sharper and more technical." {
		t.Fatalf("stored persona = %#v ok=%v", restored, ok)
	}
}

func TestDeletePersonaHandlerRemovesUnreferencedCustomPersona(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	if err := workflowStore.SavePersona(personadomain.Persona{
		ID:            "custom_writer",
		DisplayName:   "Custom Writer",
		DefaultFormat: outputformat.IDNoteArticle,
	}); err != nil {
		t.Fatalf("save persona: %v", err)
	}
	request := httptest.NewRequest(http.MethodDelete, "/api/personas/custom_writer", nil)
	request = mux.SetURLVars(request, map[string]string{"id": "custom_writer"})
	response := httptest.NewRecorder()

	DeletePersonaHandler(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if _, ok := workflowStore.GetPersona("custom_writer"); ok {
		t.Fatal("persona was not deleted")
	}
}

func TestDeletePersonaHandlerRejectsReferencedCustomPersona(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	persona := personadomain.Persona{
		ID:            "custom_writer",
		DisplayName:   "Custom Writer",
		DefaultFormat: outputformat.IDNoteArticle,
	}
	if err := workflowStore.SavePersona(persona); err != nil {
		t.Fatalf("save persona: %v", err)
	}
	session, err := briefdomain.NewArticleBriefSessionWithOptions("session-custom-persona", "profile-1", persona.ID, outputformat.IDNoteArticle, "", briefdomain.FixedQuestions())
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if err := workflowStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	request := httptest.NewRequest(http.MethodDelete, "/api/personas/custom_writer", nil)
	request = mux.SetURLVars(request, map[string]string{"id": "custom_writer"})
	response := httptest.NewRecorder()

	DeletePersonaHandler(response, request)

	assertErrorResponse(t, response, http.StatusConflict, "PERSONA_REFERENCED")
	if _, ok := workflowStore.GetPersona("custom_writer"); !ok {
		t.Fatal("referenced persona should remain")
	}
}

func TestCreatePersonaHandlerValidatesRequiredFieldsAndFormat(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{
			name:   "missing id",
			body:   `{"display_name":"Custom","default_format":"note_article"}`,
			status: http.StatusBadRequest,
			code:   "INVALID_PERSONA",
		},
		{
			name:   "missing display name",
			body:   `{"id":"custom_writer","default_format":"note_article"}`,
			status: http.StatusBadRequest,
			code:   "INVALID_PERSONA",
		},
		{
			name:   "reserved built-in id",
			body:   `{"id":"terisuke","display_name":"Custom","description":"desc","default_format":"note_article","voice_notes":{"tone":"tone"}}`,
			status: http.StatusConflict,
			code:   "PERSONA_ID_RESERVED",
		},
		{
			name:   "unknown format",
			body:   `{"id":"custom_writer","display_name":"Custom","description":"desc","default_format":"missing","voice_notes":{"tone":"tone"}}`,
			status: http.StatusBadRequest,
			code:   "UNKNOWN_OUTPUT_FORMAT",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			CreatePersonaHandler(response, httptest.NewRequest(http.MethodPost, "/api/personas", bytes.NewBufferString(tt.body)))
			assertErrorResponse(t, response, tt.status, tt.code)
		})
	}
}
