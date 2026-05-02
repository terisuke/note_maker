package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
	"github.com/teradakousuke/note_maker/internal/infrastructure/repository/memory"
)

func TestGenerateDraftHandlerStreamsSSE(t *testing.T) {
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range []string{
			"# AIと違和感を小さく言語化する\n\n",
			strings.Repeat("僕はAIと起業の挑戦について、「違和感」を言語化しながら自分の判断を見直しました。\n\n", 18),
			"## 体験から始める\n\n" + strings.Repeat("僕は音楽とエンジニアの経験を行き来し、読者が小さくアウトプットできる形にします。\n\n", 12),
			"## 次の一歩\n\n" + strings.Repeat("僕は抽象論で終わらせず、今日試せる判断基準としてAIとの向き合い方を置き直します。\n\n", 8),
		} {
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":` + quoteJSONString(chunk) + `}}]}` + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
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
	if err := workflowStore.SaveBrief("session_stream", briefdomain.ArticleBrief{
		StyleProfileID:        style.Profile.ID,
		PersonaID:             persona.ID,
		OutputFormatID:        outputformat.IDNoteArticle,
		Theme:                 "ストリーミングする",
		TargetLengthStructure: "3000字、導入・本論・結論",
	}); err != nil {
		t.Fatalf("save brief: %v", err)
	}

	body := `{"style_profile_id":"` + style.Profile.ID + `","session_id":"session_stream","persona_id":"terisuke","output_format_id":"note_article","draft_model":"gemma4:31b"}`
	request := httptest.NewRequest(http.MethodPost, "/api/drafts", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	response := httptest.NewRecorder()

	GenerateDraftHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("unexpected content type: %s", got)
	}
	stream := response.Body.String()
	for _, want := range []string{"event: status", "draft_generation_started", "event: chunk", "event: result", "event: done"} {
		if !strings.Contains(stream, want) {
			t.Fatalf("stream missing %q:\n%s", want, stream)
		}
	}
}

func quoteJSONString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return `"` + value + `"`
}
