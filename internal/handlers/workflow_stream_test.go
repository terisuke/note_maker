package handlers

import (
	"bytes"
	"encoding/json"
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
	style, err := buildPresetAuthorStyle(persona, outputformat.DefaultRegistry().MustGet(outputformat.IDNoteArticle))
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

func TestGenerateDraftHandlerStreamsFromCompletedSessionFallback(t *testing.T) {
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
		if payload.Model != "draft-test" {
			t.Fatalf("model = %q, want draft-test", payload.Model)
		}
		if !payload.Stream {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"PASS\nSummary: ブリーフに沿っています"}}]}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, chunk := range []string{
			"# Local workflow tests\n\n",
			strings.Repeat("僕はHTTPハンドラーのテストから、保存済みブリーフがなくても完了済みセッションを使って下書きを生成できることを確認しました。\n\n", 18),
			"## 具体的な検証\n\n" + strings.Repeat("僕はSSEのstatus、chunk、result、doneが順に返ることを見ながら、外部サービスなしで振る舞いを固定しました。\n\n", 12),
			"## 次の一歩\n\n" + strings.Repeat("僕は回帰を見つけやすくするため、入力の省略時にも既存セッションのpersonaとformatが使われることを残します。\n\n", 8),
		} {
			_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":` + quoteJSONString(chunk) + `}}]}` + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer llmServer.Close()
	t.Setenv("LLM_BASE_URL", llmServer.URL+"/v1")
	t.Setenv("DRAFT_LLM_MODEL", "draft-test")
	t.Setenv("VERIFY_LLM_MODEL", "draft-test")

	style := setupWorkflowStyle(t)
	session := sessionWithFixedAnswers(t, "session_stream_completed", style.Profile.ID)
	session.MarkDeepDiveSkipped()
	if _, err := session.Complete(); err != nil {
		t.Fatalf("complete session: %v", err)
	}
	if err := workflowStore.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	body := `{"style_profile_id":"` + style.Profile.ID + `","session_id":"session_stream_completed","draft_model":"draft-test","verify_model":"draft-test"}`
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
	for _, want := range []string{"event: status", "draft_generation_started", "event: chunk", "event: result", "event: done", "# Local workflow tests"} {
		if !strings.Contains(stream, want) {
			t.Fatalf("stream missing %q:\n%s", want, stream)
		}
	}
}

func TestGenerateDraftHandlerReturnsJSONDraft(t *testing.T) {
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
		if payload.Stream {
			t.Fatal("JSON draft path should not request streaming")
		}
		switch payload.Model {
		case "draft-json-test":
			content := "# Local workflow tests\n\n" +
				strings.Repeat("僕は保存済みブリーフを使い、通常のJSONレスポンスでも下書きが返ることを確認しました。\n\n", 18) +
				"## 検証\n\n" +
				strings.Repeat("僕はpersonaとformatをブリーフから復元し、Note向けMarkdownとして扱える形に整えます。\n\n", 12) +
				"## 次の一歩\n\n" +
				strings.Repeat("僕はこのテストでSSE以外の生成経路も固定し、UI追加前の回帰を抑えます。\n\n", 8)
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":` + quoteJSONString(content) + `}}]}`))
		case "verify-json-test":
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"PASS\nSummary: 文体とブリーフに沿っています"}}]}`))
		default:
			t.Fatalf("unexpected model: %s", payload.Model)
		}
	}))
	defer llmServer.Close()
	t.Setenv("LLM_BASE_URL", llmServer.URL+"/v1")
	t.Setenv("DRAFT_LLM_MODEL", "draft-json-test")
	t.Setenv("VERIFY_LLM_MODEL", "verify-json-test")

	style := setupWorkflowStyle(t)
	brief := briefdomain.ArticleBrief{
		StyleProfileID:        style.Profile.ID,
		PersonaID:             personadomain.IDTerisuke,
		OutputFormatID:        outputformat.IDNoteArticle,
		Theme:                 "JSONで下書きを返す",
		Reader:                "HTTP handlerを保守する開発者",
		MustInclude:           "通常レスポンス、検証、文体評価",
		TargetLengthStructure: "2500字前後",
		ToneStance:            "内省的だが実装に寄せる",
	}
	if err := workflowStore.SaveBrief("session_json_draft", brief); err != nil {
		t.Fatalf("save brief: %v", err)
	}

	body := `{"style_profile_id":"` + style.Profile.ID + `","session_id":"session_json_draft","draft_model":"draft-json-test","verify_model":"verify-json-test"}`
	request := httptest.NewRequest(http.MethodPost, "/api/drafts", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	GenerateDraftHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload generateDraftResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(payload.Draft, "# Local workflow tests") || !payload.Verification.Performed {
		t.Fatalf("unexpected draft response: %#v", payload)
	}
}

func quoteJSONString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return `"` + value + `"`
}
