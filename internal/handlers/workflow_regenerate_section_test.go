package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
	"github.com/teradakousuke/note_maker/internal/infrastructure/repository/memory"
)

func TestRegenerateDraftSectionHandlerReplacesOnlyTargetSection(t *testing.T) {
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"## 実装\n\n新しい実装内容です。"}}]}`))
	}))
	defer llmServer.Close()
	t.Setenv("LLM_BASE_URL", llmServer.URL+"/v1")
	t.Setenv("DRAFT_LLM_MODEL", "gemma4:31b")

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
	if err := workflowStore.SaveBrief("session-1", briefdomain.ArticleBrief{
		StyleProfileID:        style.Profile.ID,
		PersonaID:             persona.ID,
		OutputFormatID:        outputformat.IDNoteArticle,
		Theme:                 "セクション再生成",
		Reader:                "記事を書く人",
		MustInclude:           "実装",
		TargetLengthStructure: "1200字",
	}); err != nil {
		t.Fatalf("save brief: %v", err)
	}

	draft := "# Title\n\nIntro\n\n## 実装\n\n古い実装内容です。\n\n## 検証\n\nここは残します。\n"
	body := `{"style_profile_id":"` + style.Profile.ID + `","session_id":"session-1","section_anchor":"実装","draft_markdown":` + quoteJSONString(draft) + `}`
	request := httptest.NewRequest(http.MethodPost, "/api/drafts/session-1/regenerate-section", bytes.NewBufferString(body))
	request = mux.SetURLVars(request, map[string]string{"id": "session-1"})
	response := httptest.NewRecorder()

	RegenerateDraftSectionHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload regenerateDraftSectionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(payload.UpdatedDraftMarkdown, "## 実装\n\n新しい実装内容です。") {
		t.Fatalf("updated draft missing replacement:\n%s", payload.UpdatedDraftMarkdown)
	}
	if !strings.Contains(payload.UpdatedDraftMarkdown, "## 検証\n\nここは残します。") {
		t.Fatalf("non-target section changed:\n%s", payload.UpdatedDraftMarkdown)
	}
	if strings.Contains(payload.UpdatedDraftMarkdown, "古い実装内容") {
		t.Fatalf("old section remained:\n%s", payload.UpdatedDraftMarkdown)
	}
}
