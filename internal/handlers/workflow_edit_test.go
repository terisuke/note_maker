package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	"github.com/teradakousuke/note_maker/internal/infrastructure/repository/memory"
)

func TestEditBriefAnswerHandlerForksSession(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	session, err := briefdomain.NewArticleBriefSession("session-1", "style-1")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	answers := []string{
		"Local article generation.",
		"Open with a failed local LLM run.",
		"Solo developers.",
		"Try a three-phase workflow.",
		"Mention final checks.",
	}
	for _, answer := range answers {
		if _, err := session.RecordAnswer(answer); err != nil {
			t.Fatalf("record answer: %v", err)
		}
	}
	if err := workflowStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	body := bytes.NewBufferString(`{"content":"Edited opening","brief_model":""}`)
	request := httptest.NewRequest(http.MethodPost, "/api/brief-sessions/session-1/answers/opening_episode/edit", body)
	request = mux.SetURLVars(request, map[string]string{
		"id":        "session-1",
		"answer_id": briefdomain.QuestionIDOpeningEpisode,
	})
	response := httptest.NewRecorder()

	EditBriefAnswerHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload briefSessionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.SessionID == "session-1" || payload.ParentSessionID != "session-1" {
		t.Fatalf("unexpected session lineage: %#v", payload)
	}
	if len(payload.Answers) != 2 {
		t.Fatalf("answers = %d, want 2", len(payload.Answers))
	}
	if payload.Answers[1].QuestionID != briefdomain.QuestionIDOpeningEpisode || payload.Answers[1].Content != "Edited opening" {
		t.Fatalf("edited answer = %#v", payload.Answers[1])
	}
	original, ok := workflowStore.GetSession("session-1")
	if !ok {
		t.Fatal("original session was not retained")
	}
	if original.Answers[1].Content != answers[1] {
		t.Fatalf("original answer changed: %#v", original.Answers[1])
	}
	if _, ok := workflowStore.GetSession(payload.SessionID); !ok {
		t.Fatal("forked session was not saved")
	}
}

func TestEditBriefAnswerHandlerValidatesStoredSessionAndAnswer(t *testing.T) {
	workflowStore = memory.NewWorkflowStore()
	session, err := briefdomain.NewArticleBriefSession("session-1", "style-1")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if _, err := session.RecordAnswer("Local article generation."); err != nil {
		t.Fatalf("record answer: %v", err)
	}
	if err := workflowStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	tests := []struct {
		name      string
		sessionID string
		answerID  string
		status    int
		code      string
	}{
		{
			name:      "missing session",
			sessionID: "missing",
			answerID:  briefdomain.QuestionIDTheme,
			status:    http.StatusNotFound,
			code:      "BRIEF_SESSION_NOT_FOUND",
		},
		{
			name:      "missing answer",
			sessionID: "session-1",
			answerID:  briefdomain.QuestionIDOpeningEpisode,
			status:    http.StatusBadRequest,
			code:      "BRIEF_ANSWER_EDIT_FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bytes.NewBufferString(`{"content":"Edited answer"}`)
			request := httptest.NewRequest(http.MethodPost, "/api/brief-sessions/"+tt.sessionID+"/answers/"+tt.answerID+"/edit", body)
			request = mux.SetURLVars(request, map[string]string{
				"id":        tt.sessionID,
				"answer_id": tt.answerID,
			})
			response := httptest.NewRecorder()

			EditBriefAnswerHandler(response, request)

			assertErrorResponse(t, response, tt.status, tt.code)
		})
	}
}

func TestUpdateBriefArtifactHandlerUpdatesSavedBriefWithoutRewritingSessionHistory(t *testing.T) {
	style := setupWorkflowStyle(t)
	session := sessionWithFixedAnswers(t, "session-brief-edit", style.Profile.ID)
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
	originalThemeAnswer := session.Answers[0].Content

	body := bytes.NewBufferString(`{"fields":{"theme":"Edited saved theme","tone_stance":"Edited saved tone"}}`)
	request := httptest.NewRequest(http.MethodPatch, "/api/briefs/session-brief-edit", body)
	request = mux.SetURLVars(request, map[string]string{"id": session.ID})
	response := httptest.NewRecorder()

	UpdateBriefArtifactHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload updateBriefResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Brief.Brief.Theme != "Edited saved theme" || payload.Brief.Brief.ToneStance != "Edited saved tone" {
		t.Fatalf("brief was not updated: %#v", payload.Brief.Brief)
	}
	savedBrief, ok := workflowStore.GetBrief(session.ID)
	if !ok || savedBrief.Theme != "Edited saved theme" {
		t.Fatalf("saved brief = %#v ok=%v", savedBrief, ok)
	}
	savedSession, ok := workflowStore.GetSession(session.ID)
	if !ok {
		t.Fatal("session missing after brief edit")
	}
	if savedSession.Answers[0].Content != originalThemeAnswer {
		t.Fatalf("session answer history was rewritten: %#v", savedSession.Answers[0])
	}
}

func TestUpdateBriefArtifactHandlerValidatesFields(t *testing.T) {
	style := setupWorkflowStyle(t)
	if err := workflowStore.SaveBrief("session-brief-edit", briefdomain.ArticleBrief{
		StyleProfileID:        style.Profile.ID,
		PersonaID:             "terisuke",
		OutputFormatID:        "note_article",
		Theme:                 "Theme",
		Reader:                "Reader",
		ExpectedReaderAction:  "Action",
		MustInclude:           "Include",
		PersonalContext:       "Context",
		TargetLengthStructure: "1200字",
	}); err != nil {
		t.Fatalf("save brief: %v", err)
	}

	request := httptest.NewRequest(http.MethodPatch, "/api/briefs/session-brief-edit", bytes.NewBufferString(`{"fields":{"unknown":"value"}}`))
	request = mux.SetURLVars(request, map[string]string{"id": "session-brief-edit"})
	response := httptest.NewRecorder()

	UpdateBriefArtifactHandler(response, request)

	assertErrorResponse(t, response, http.StatusBadRequest, "INVALID_BRIEF_UPDATE")
}

func TestCreateStyleGuideVersionHandlerAppendsEditableVersion(t *testing.T) {
	style := setupWorkflowStyle(t)
	body := bytes.NewBufferString(`{"guide_markdown":"- 一人称: 私\n- よく扱うテーマ: local workflow\n- 段落のリズム: short\n- 文のリズム: direct\n- 見出し: clear\n- 引用表現: sparing\n- 書き出し: concrete\n- 締め方: next action"}`)
	request := httptest.NewRequest(http.MethodPatch, "/api/author-style/"+style.Profile.ID, body)
	request = mux.SetURLVars(request, map[string]string{"id": style.Profile.ID})
	response := httptest.NewRecorder()

	CreateStyleGuideVersionHandler(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload styleGuideArtifactResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.AnalysisID == style.ID || payload.GuideID == style.Guide.ID {
		t.Fatalf("style guide version did not get new ids: %#v", payload)
	}
	if payload.ProfileID != style.Profile.ID || payload.GuideMarkdown == style.Guide.Markdown {
		t.Fatalf("unexpected style guide version: %#v", payload)
	}
	_, latestGuide, ok := workflowStore.GetProfileAndGuide(style.Profile.ID)
	if !ok {
		t.Fatal("updated profile lookup missing")
	}
	if latestGuide.ID != payload.GuideID {
		t.Fatalf("profile lookup did not point to latest guide: %s want %s", latestGuide.ID, payload.GuideID)
	}
}
