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
