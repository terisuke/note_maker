package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	authorstyleapp "github.com/teradakousuke/note_maker/internal/application/authorstyle"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
	"github.com/teradakousuke/note_maker/internal/infrastructure/repository/memory"
)

func TestSeedAuthorStyleHandlerStoresPresetAndGetAuthorStyle(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()

	request := httptest.NewRequest(http.MethodPost, "/api/author-styles/seed", bytes.NewBufferString(`{"persona_id":"terisuke"}`))
	response := httptest.NewRecorder()

	SeedAuthorStyleHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("seed status = %d, body = %s", response.Code, response.Body.String())
	}
	var seeded authorStyleResponse
	if err := json.NewDecoder(response.Body).Decode(&seeded); err != nil {
		t.Fatalf("decode seed response: %v", err)
	}
	if seeded.ProfileID == "" || seeded.GuideID == "" || !strings.Contains(seeded.GuideMarkdown, "一人称") {
		t.Fatalf("unexpected seeded style response: %#v", seeded)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/author-styles/"+seeded.ProfileID, nil)
	getRequest = mux.SetURLVars(getRequest, map[string]string{"id": seeded.ProfileID})
	getResponse := httptest.NewRecorder()

	GetAuthorStyleHandler(getResponse, getRequest)

	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getResponse.Code, getResponse.Body.String())
	}
	var fetched authorStyleResponse
	if err := json.NewDecoder(getResponse.Body).Decode(&fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.ID != seeded.ID || fetched.ProfileID != seeded.ProfileID {
		t.Fatalf("fetched style = %#v, seeded = %#v", fetched, seeded)
	}
}

func TestSeedAuthorStyleHandlerRejectsUnknownPersona(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	request := httptest.NewRequest(http.MethodPost, "/api/author-styles/seed", bytes.NewBufferString(`{"persona_id":"missing"}`))
	response := httptest.NewRecorder()

	SeedAuthorStyleHandler(response, request)

	assertErrorResponse(t, response, http.StatusBadRequest, "UNKNOWN_PERSONA")
}

func TestGetAuthorStyleHandlerNotFound(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	request := httptest.NewRequest(http.MethodGet, "/api/author-styles/missing", nil)
	request = mux.SetURLVars(request, map[string]string{"id": "missing"})
	response := httptest.NewRecorder()

	GetAuthorStyleHandler(response, request)

	assertErrorResponse(t, response, http.StatusNotFound, "AUTHOR_STYLE_NOT_FOUND")
}

func TestCreateBriefSessionHandlerCreatesAndPersistsSession(t *testing.T) {
	style := setupWorkflowStyle(t)
	body := `{"style_profile_id":"` + style.Profile.ID + `","session_id":"session-create"}`
	request := httptest.NewRequest(http.MethodPost, "/api/brief-sessions", bytes.NewBufferString(body))
	response := httptest.NewRecorder()

	CreateBriefSessionHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload briefSessionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.SessionID != "session-create" || payload.StyleProfileID != style.Profile.ID {
		t.Fatalf("unexpected session response: %#v", payload)
	}
	if payload.PersonaID != personadomain.IDTerisuke || payload.OutputFormatID != outputformat.IDNoteArticle {
		t.Fatalf("unexpected defaults: %#v", payload)
	}
	if payload.NextQuestion == nil || payload.NextQuestion.ID != briefdomain.QuestionIDTheme {
		t.Fatalf("next question = %#v", payload.NextQuestion)
	}
	if _, ok := workflowStore.GetSession("session-create"); !ok {
		t.Fatal("created session was not saved")
	}
}

func TestCreateBriefSessionHandlerValidatesInputs(t *testing.T) {
	style := setupWorkflowStyle(t)
	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{
			name:   "missing style",
			body:   `{}`,
			status: http.StatusBadRequest,
			code:   "MISSING_REQUIRED_FIELD",
		},
		{
			name:   "unknown style",
			body:   `{"style_profile_id":"missing","session_id":"session-unknown-style"}`,
			status: http.StatusNotFound,
			code:   "AUTHOR_STYLE_NOT_FOUND",
		},
		{
			name:   "unknown persona",
			body:   `{"style_profile_id":"` + style.Profile.ID + `","session_id":"session-unknown-persona","persona_id":"missing"}`,
			status: http.StatusBadRequest,
			code:   "UNKNOWN_PERSONA",
		},
		{
			name:   "unknown output format",
			body:   `{"style_profile_id":"` + style.Profile.ID + `","session_id":"session-unknown-format","output_format_id":"missing"}`,
			status: http.StatusBadRequest,
			code:   "UNKNOWN_OUTPUT_FORMAT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/brief-sessions", bytes.NewBufferString(tt.body))
			response := httptest.NewRecorder()

			CreateBriefSessionHandler(response, request)

			assertErrorResponse(t, response, tt.status, tt.code)
		})
	}
}

func TestGetBriefSessionHandlerReturnsStoredProgressAndCompletedBrief(t *testing.T) {
	style := setupWorkflowStyle(t)
	session := sessionWithFixedAnswers(t, "session-completed", style.Profile.ID)
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
	request := httptest.NewRequest(http.MethodGet, "/api/brief-sessions/session-completed", nil)
	request = mux.SetURLVars(request, map[string]string{"id": "session-completed"})
	response := httptest.NewRecorder()

	GetBriefSessionHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload briefSessionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Completed || payload.Brief == nil || payload.Brief.Theme != "Local workflow tests" {
		t.Fatalf("unexpected completed response: %#v", payload)
	}
}

func TestAnswerBriefSessionHandlerRecordsAnswerWithoutLLM(t *testing.T) {
	style := setupWorkflowStyle(t)
	session, err := briefdomain.NewArticleBriefSession("session-answer", style.Profile.ID)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if err := workflowStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/brief-sessions/session-answer/answers", bytes.NewBufferString(`{"content":"Local workflow coverage"}`))
	request = mux.SetURLVars(request, map[string]string{"id": "session-answer"})
	response := httptest.NewRecorder()

	AnswerBriefSessionHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload briefSessionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Completed || payload.NextQuestion == nil || payload.NextQuestion.ID != briefdomain.QuestionIDOpeningEpisode {
		t.Fatalf("unexpected answer response: %#v", payload)
	}
	saved, ok := workflowStore.GetSession("session-answer")
	if !ok {
		t.Fatal("answered session was not saved")
	}
	if len(saved.Answers) != 1 || saved.Answers[0].QuestionID != briefdomain.QuestionIDTheme {
		t.Fatalf("saved answers = %#v", saved.Answers)
	}
}

func TestAnswerBriefSessionHandlerSkipDeepDiveCompletesAndSavesBrief(t *testing.T) {
	style := setupWorkflowStyle(t)
	session := sessionWithFixedAnswers(t, "session-skip", style.Profile.ID)
	if err := workflowStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/brief-sessions/session-skip/answers", bytes.NewBufferString(`{"skip_deep_dive":true}`))
	request = mux.SetURLVars(request, map[string]string{"id": "session-skip"})
	response := httptest.NewRecorder()

	AnswerBriefSessionHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload briefSessionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Completed || payload.Brief == nil || payload.Brief.Theme != "Local workflow tests" {
		t.Fatalf("unexpected skip response: %#v", payload)
	}
	if savedBrief, ok := workflowStore.GetBrief("session-skip"); !ok || savedBrief.Theme != "Local workflow tests" {
		t.Fatalf("saved brief = %#v, ok = %v", savedBrief, ok)
	}
}

func TestAnswerBriefSessionHandlerStreamsFirstAnswerWithoutLLM(t *testing.T) {
	style := setupWorkflowStyle(t)
	session, err := briefdomain.NewArticleBriefSession("session-stream-answer", style.Profile.ID)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if err := workflowStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/brief-sessions/session-stream-answer/answers", bytes.NewBufferString(`{"content":"Streaming answer"}`))
	request.Header.Set("Accept", "text/event-stream")
	request = mux.SetURLVars(request, map[string]string{"id": "session-stream-answer"})
	response := httptest.NewRecorder()

	AnswerBriefSessionHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("unexpected content type: %s", got)
	}
	stream := response.Body.String()
	for _, want := range []string{"event: status", "stream_opened", "event: result", "event: done"} {
		if !strings.Contains(stream, want) {
			t.Fatalf("stream missing %q:\n%s", want, stream)
		}
	}
}

func TestAnswerBriefSessionHandlerRejectsMissingSession(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	request := httptest.NewRequest(http.MethodPost, "/api/brief-sessions/missing/answers", bytes.NewBufferString(`{"content":"answer"}`))
	request = mux.SetURLVars(request, map[string]string{"id": "missing"})
	response := httptest.NewRecorder()

	AnswerBriefSessionHandler(response, request)

	assertErrorResponse(t, response, http.StatusNotFound, "BRIEF_SESSION_NOT_FOUND")
}

func TestGenerateDraftHandlerValidatesStoredContextBeforeLLM(t *testing.T) {
	style := setupWorkflowStyle(t)
	if err := workflowStore.SaveBrief("session-draft", briefdomain.ArticleBrief{
		StyleProfileID:        style.Profile.ID,
		PersonaID:             personadomain.IDTerisuke,
		OutputFormatID:        outputformat.IDNoteArticle,
		Theme:                 "Draft validation",
		Reader:                "Developers",
		MustInclude:           "No LLM call",
		TargetLengthStructure: "1200字",
	}); err != nil {
		t.Fatalf("save brief: %v", err)
	}
	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{
			name:   "style not found",
			body:   `{"style_profile_id":"missing","session_id":"session-draft"}`,
			status: http.StatusNotFound,
			code:   "AUTHOR_STYLE_NOT_FOUND",
		},
		{
			name:   "brief not found",
			body:   `{"style_profile_id":"` + style.Profile.ID + `","session_id":"missing"}`,
			status: http.StatusBadRequest,
			code:   "BRIEF_NOT_FOUND",
		},
		{
			name:   "unknown persona",
			body:   `{"style_profile_id":"` + style.Profile.ID + `","session_id":"session-draft","persona_id":"missing"}`,
			status: http.StatusBadRequest,
			code:   "UNKNOWN_PERSONA",
		},
		{
			name:   "unknown output format",
			body:   `{"style_profile_id":"` + style.Profile.ID + `","session_id":"session-draft","output_format_id":"missing"}`,
			status: http.StatusBadRequest,
			code:   "UNKNOWN_OUTPUT_FORMAT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/drafts", bytes.NewBufferString(tt.body))
			response := httptest.NewRecorder()

			GenerateDraftHandler(response, request)

			assertErrorResponse(t, response, tt.status, tt.code)
		})
	}
}

func TestRegenerateDraftSectionHandlerValidatesContextBeforeLLM(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	request := httptest.NewRequest(http.MethodPost, "/api/drafts/missing/regenerate-section", bytes.NewBufferString(`{"draft_markdown":"# Title","section_anchor":"Title"}`))
	request = mux.SetURLVars(request, map[string]string{"id": "missing"})
	response := httptest.NewRecorder()

	RegenerateDraftSectionHandler(response, request)

	assertErrorResponse(t, response, http.StatusBadRequest, "DRAFT_CONTEXT_NOT_FOUND")
}

func setupWorkflowStyle(t *testing.T) authorstyleapp.AnalyzeResult {
	t.Helper()
	workflowStore = memory.NewWorkflowStore()
	persona, ok := personadomain.DefaultRegistry().Get(personadomain.IDTerisuke)
	if !ok {
		t.Fatal("missing terisuke persona")
	}
	style, err := buildPresetAuthorStyle(persona)
	if err != nil {
		t.Fatalf("build style: %v", err)
	}
	if err := workflowStore.SaveAuthorStyle(style); err != nil {
		t.Fatalf("save style: %v", err)
	}
	return style
}

func sessionWithFixedAnswers(t *testing.T, id, styleProfileID string) briefdomain.ArticleBriefSession {
	t.Helper()
	session, err := briefdomain.NewArticleBriefSession(id, styleProfileID)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	answers := []string{
		"Local workflow tests",
		"A handler test failed before reaching a networked LLM.",
		"Maintainers adding coverage.",
		"Keep endpoint behavior stable.",
		"HTTP status codes and persistence checks.",
		"Testing the workflow handler without external services.",
		"Avoid broad source changes.",
		"1200字, validation focused.",
		"Practical and concise.",
	}
	for _, answer := range answers {
		if _, err := session.RecordAnswer(answer); err != nil {
			t.Fatalf("record answer %q: %v", answer, err)
		}
	}
	return session
}

func assertErrorResponse(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, status, response.Body.String())
	}
	var payload ErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Error.Code != code {
		t.Fatalf("error code = %q, want %q; payload = %#v", payload.Error.Code, code, payload)
	}
}
