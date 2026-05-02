package handlers

import (
	"bytes"
	"context"
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

func TestSeedAuthorStyleHandlerRejectsInvalidJSON(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	request := httptest.NewRequest(http.MethodPost, "/api/author-styles/seed", bytes.NewBufferString(`{`))
	response := httptest.NewRecorder()

	SeedAuthorStyleHandler(response, request)

	assertErrorResponse(t, response, http.StatusBadRequest, "INVALID_REQUEST_FORMAT")
}

func TestRefineStyleGuideWithModelUsesStyleRuntime(t *testing.T) {
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload struct {
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode llm request: %v", err)
		}
		if payload.Model != "style-test" {
			t.Fatalf("model = %q, want style-test", payload.Model)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"- 導入は具体的な違和感から始める\n- 締めは読者の次の一歩に寄せる"}}]}`))
	}))
	defer llmServer.Close()
	t.Setenv("LLM_BASE_URL", llmServer.URL+"/v1")

	result := setupWorkflowStyle(t)
	refined, err := refineStyleGuideWithModel(context.Background(), result, "style-test")
	if err != nil {
		t.Fatalf("refine style guide: %v", err)
	}
	if !strings.Contains(refined, "具体的な違和感") {
		t.Fatalf("unexpected refined guide: %s", refined)
	}
}

func TestAnalyzeAuthorStyleHandlerFetchesHTMLArticle(t *testing.T) {
	articleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>HTML article</title></head><body><article><h1>HTML article</h1>` +
			strings.Repeat(`<p>僕はHTML記事から文体を抽出し、過去記事の調子を保存する流れを確認します。</p>`, 12) +
			`</article></body></html>`))
	}))
	defer articleServer.Close()
	workflowStore = memory.NewWorkflowStore()

	body := `{"article_urls":[` + quoteJSONString(articleServer.URL+"/article") + `],"limit":1}`
	request := httptest.NewRequest(http.MethodPost, "/api/author-styles/analyze", bytes.NewBufferString(body))
	response := httptest.NewRecorder()

	AnalyzeAuthorStyleHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload authorStyleResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ProfileID == "" || payload.GuideID == "" || payload.ArticleCount != 1 {
		t.Fatalf("unexpected analysis response: %#v", payload)
	}
	if _, ok := workflowStore.GetAuthorStyle(payload.ID); !ok {
		t.Fatal("analysis result was not saved")
	}
}

func TestAnalyzeAuthorStyleHandlerValidatesRequest(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	tests := []struct {
		name   string
		body   string
		status int
		code   string
	}{
		{name: "bad json", body: `{`, status: http.StatusBadRequest, code: "INVALID_REQUEST_FORMAT"},
		{name: "missing source", body: `{}`, status: http.StatusInternalServerError, code: "AUTHOR_STYLE_ANALYSIS_FAILED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/author-styles/analyze", bytes.NewBufferString(tt.body))
			response := httptest.NewRecorder()

			AnalyzeAuthorStyleHandler(response, request)

			assertErrorResponse(t, response, tt.status, tt.code)
		})
	}
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

func TestCreateBriefSessionHandlerAppendsCustomQuestionsAfterTemplate(t *testing.T) {
	style := setupWorkflowStyle(t)
	body := `{"style_profile_id":"` + style.Profile.ID + `","session_id":"session-custom","persona_id":"cloudia","output_format_id":"zenn_article","questions":[{"id":"custom_reference","text":"参考リンクとして必ず確認するURLは何ですか？"}]}`
	request := httptest.NewRequest(http.MethodPost, "/api/brief-sessions", bytes.NewBufferString(body))
	response := httptest.NewRecorder()

	CreateBriefSessionHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	session, ok := workflowStore.GetSession("session-custom")
	if !ok {
		t.Fatal("created session was not saved")
	}
	template := briefdomain.ComposeFixedQuestions(personadomain.IDCloudia, outputformat.IDZennArticle)
	if len(session.Questions) != len(template)+1 {
		t.Fatalf("question count = %d, want %d", len(session.Questions), len(template)+1)
	}
	if session.Questions[len(template)-1].ID != briefdomain.QuestionIDCloudiaViewpoint {
		t.Fatalf("last template question = %#v", session.Questions[len(template)-1])
	}
	if session.Questions[len(session.Questions)-1].ID != "custom_reference" {
		t.Fatalf("custom question was not appended after template: %#v", session.Questions)
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

func TestGetBriefSessionHandlerRejectsMissingSession(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	request := httptest.NewRequest(http.MethodGet, "/api/brief-sessions/missing", nil)
	request = mux.SetURLVars(request, map[string]string{"id": "missing"})
	response := httptest.NewRecorder()

	GetBriefSessionHandler(response, request)

	assertErrorResponse(t, response, http.StatusNotFound, "BRIEF_SESSION_NOT_FOUND")
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

func TestAnswerBriefSessionHandlerValidatesRequestAndSessionState(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	tests := []struct {
		name      string
		sessionID string
		body      string
		status    int
		code      string
		setup     func(t *testing.T)
	}{
		{
			name:      "bad json",
			sessionID: "session-any",
			body:      `{`,
			status:    http.StatusBadRequest,
			code:      "INVALID_REQUEST_FORMAT",
		},
		{
			name:      "missing session",
			sessionID: "missing",
			body:      `{"content":"answer"}`,
			status:    http.StatusNotFound,
			code:      "BRIEF_SESSION_NOT_FOUND",
		},
		{
			name:      "skip before complete",
			sessionID: "session-incomplete",
			body:      `{"skip_deep_dive":true}`,
			status:    http.StatusBadRequest,
			code:      "BRIEF_SESSION_INCOMPLETE",
			setup: func(t *testing.T) {
				style := setupWorkflowStyle(t)
				session, err := briefdomain.NewArticleBriefSession("session-incomplete", style.Profile.ID)
				if err != nil {
					t.Fatalf("new session: %v", err)
				}
				if err := workflowStore.SaveSession(session); err != nil {
					t.Fatalf("save session: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflowStore = memory.NewWorkflowStore()
			if tt.setup != nil {
				tt.setup(t)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/brief-sessions/"+tt.sessionID+"/answers", bytes.NewBufferString(tt.body))
			request = mux.SetURLVars(request, map[string]string{"id": tt.sessionID})
			response := httptest.NewRecorder()

			AnswerBriefSessionHandler(response, request)

			assertErrorResponse(t, response, tt.status, tt.code)
		})
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

func TestAnswerBriefSessionHandlerStreamsGeneratedFollowUp(t *testing.T) {
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode llm request: %v", err)
		}
		if payload.Model != "brief-test" {
			t.Fatalf("model = %q, want brief-test", payload.Model)
		}
		if !payload.Stream {
			t.Fatal("follow-up generation should use streaming completions")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range []string{
			"「A handler test failed before reaching a networked LLM.」というご回答を踏まえて、",
			"どの判断を最初に説明しますか？",
		} {
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":` + quoteJSONString(chunk) + `}}]}` + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer llmServer.Close()
	t.Setenv("LLM_BASE_URL", llmServer.URL+"/v1")
	t.Setenv("BRIEF_LLM_MODEL", "brief-test")

	style := setupWorkflowStyle(t)
	session := sessionWithFixedAnswers(t, "session-follow-up-stream", style.Profile.ID)
	if err := workflowStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/brief-sessions/session-follow-up-stream/answers", bytes.NewBufferString(`{"content":"The first follow-up answer adds the concrete decision point."}`))
	request.Header.Set("Accept", "text/event-stream")
	request = mux.SetURLVars(request, map[string]string{"id": "session-follow-up-stream"})
	response := httptest.NewRecorder()

	AnswerBriefSessionHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("unexpected content type: %s", got)
	}
	stream := response.Body.String()
	for _, want := range []string{"event: status", "follow_up_generation_started", "event: chunk", "event: result", "event: done"} {
		if !strings.Contains(stream, want) {
			t.Fatalf("stream missing %q:\n%s", want, stream)
		}
	}
	if !strings.Contains(stream, "どの判断を最初に説明しますか？") {
		t.Fatalf("stream missing generated follow-up question:\n%s", stream)
	}
	saved, ok := workflowStore.GetSession("session-follow-up-stream")
	if !ok {
		t.Fatal("answered session was not saved")
	}
	if got := len(saved.DeepDiveAnswers()); got != 1 {
		t.Fatalf("deep dive answer count = %d, want 1; answers = %#v", got, saved.Answers)
	}
}

func TestLLMFollowUpGeneratorUsesNonStreamingRuntime(t *testing.T) {
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode llm request: %v", err)
		}
		if payload.Model != "brief-nonstream-test" {
			t.Fatalf("model = %q, want brief-nonstream-test", payload.Model)
		}
		if payload.Stream {
			t.Fatal("non-stream follow-up should not request SSE")
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"「Local workflow tests」というご回答を踏まえて、どの判断を一番詳しく説明しますか？"}}]}`))
	}))
	defer llmServer.Close()
	t.Setenv("LLM_BASE_URL", llmServer.URL+"/v1")

	style := setupWorkflowStyle(t)
	session := sessionWithFixedAnswers(t, "session-follow-up-nonstream", style.Profile.ID)
	target := session.Questions[0]
	answer, ok := session.AnswerForQuestion(target.ID)
	if !ok {
		t.Fatalf("missing answer for %s", target.ID)
	}
	generator := llmFollowUpGenerator{model: "brief-nonstream-test", styleGuideMarkdown: style.Guide.Markdown}

	question, err := generator.GenerateFollowUp(context.Background(), session, target, answer, 1)
	if err != nil {
		t.Fatalf("generate follow-up: %v", err)
	}
	if !strings.Contains(question, "どの判断") {
		t.Fatalf("unexpected question: %s", question)
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
