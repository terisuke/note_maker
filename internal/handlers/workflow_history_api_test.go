package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
	sourcedomain "github.com/teradakousuke/note_maker/internal/domain/source"
	"github.com/teradakousuke/note_maker/internal/infrastructure/repository/memory"
	sqliterepo "github.com/teradakousuke/note_maker/internal/infrastructure/repository/sqlite"
)

func TestListProjectsHandlerReturnsSQLiteProjects(t *testing.T) {
	setupHistoryAPIStore(t)

	response := httptest.NewRecorder()
	ListProjectsHandler(response, httptest.NewRequest(http.MethodGet, "/api/projects", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	payload := decodeHistoryAPIPayload(t, response)
	projects := historyAPIArray(t, payload, "projects")
	if len(projects) != 1 {
		t.Fatalf("projects = %d, want 1: %#v", len(projects), payload)
	}
	project := historyAPIObject(t, projects[0])
	if got := historyAPIString(project, "id", "ID"); got != "project-history" {
		t.Fatalf("project id = %q, want project-history: %#v", got, project)
	}
	if got := historyAPIString(project, "name", "Name"); got != "History project" {
		t.Fatalf("project name = %q, want History project: %#v", got, project)
	}
}

func TestListProjectsHandlerReturnsEmptyListForUnsupportedStore(t *testing.T) {
	previous := workflowStore
	workflowStore = memory.NewWorkflowStore()
	t.Cleanup(func() { workflowStore = previous })

	response := httptest.NewRecorder()
	ListProjectsHandler(response, httptest.NewRequest(http.MethodGet, "/api/projects", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	payload := decodeHistoryAPIPayload(t, response)
	if projects := historyAPIArray(t, payload, "projects"); len(projects) != 0 {
		t.Fatalf("projects = %d, want 0: %#v", len(projects), payload)
	}
}

func TestListWorkflowArtifactsHandlerIncludesSQLiteHistory(t *testing.T) {
	setupHistoryAPIStore(t)

	response := httptest.NewRecorder()
	ListWorkflowArtifactsHandler(response, httptest.NewRequest(http.MethodGet, "/api/workflow/artifacts", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload workflowArtifactsResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Projects) != 1 || payload.Projects[0].ID != "project-history" {
		t.Fatalf("unexpected projects: %#v", payload.Projects)
	}
	if len(payload.Articles) != 1 || payload.Articles[0].ID != "article-history" {
		t.Fatalf("unexpected articles: %#v", payload.Articles)
	}
	if len(payload.Drafts) != 1 || payload.Drafts[0].ID != "draft-history" {
		t.Fatalf("unexpected drafts: %#v", payload.Drafts)
	}
	if len(payload.Projects[0].Articles) != 1 || len(payload.Articles[0].Drafts) != 1 || len(payload.Drafts[0].SectionRegenerations) != 1 {
		t.Fatalf("workflow artifacts missing nested history: %#v", payload)
	}
}

func TestGetProjectHandlerReturnsArticlesAndSourceSnapshots(t *testing.T) {
	setupHistoryAPIStore(t)

	request := httptest.NewRequest(http.MethodGet, "/api/projects/project-history", nil)
	request = mux.SetURLVars(request, map[string]string{"id": "project-history"})
	response := httptest.NewRecorder()
	GetProjectHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	project := historyAPINamedObject(t, decodeHistoryAPIPayload(t, response), "project")
	if got := historyAPIString(project, "id", "ID"); got != "project-history" {
		t.Fatalf("project id = %q, want project-history: %#v", got, project)
	}
	articles := historyAPIArrayFromObject(t, project, "articles")
	if len(articles) != 1 || historyAPIString(historyAPIObject(t, articles[0]), "id", "ID") != "article-history" {
		t.Fatalf("unexpected project articles: %#v", articles)
	}
	snapshots := historyAPIArrayFromObject(t, project, "source_snapshots", "SourceSnapshots")
	if len(snapshots) != 1 || historyAPIString(historyAPIObject(t, snapshots[0]), "id", "ID") != "snapshot-project" {
		t.Fatalf("unexpected project source snapshots: %#v", snapshots)
	}
}

func TestGetArticleHandlerReturnsDraftsAndSourceSnapshots(t *testing.T) {
	setupHistoryAPIStore(t)

	request := httptest.NewRequest(http.MethodGet, "/api/articles/article-history", nil)
	request = mux.SetURLVars(request, map[string]string{"id": "article-history"})
	response := httptest.NewRecorder()
	GetArticleHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	article := historyAPINamedObject(t, decodeHistoryAPIPayload(t, response), "article")
	if got := historyAPIString(article, "id", "ID"); got != "article-history" {
		t.Fatalf("article id = %q, want article-history: %#v", got, article)
	}
	drafts := historyAPIArrayFromObject(t, article, "drafts")
	if len(drafts) != 1 || historyAPIString(historyAPIObject(t, drafts[0]), "id", "ID") != "draft-history" {
		t.Fatalf("unexpected article drafts: %#v", drafts)
	}
	snapshots := historyAPIArrayFromObject(t, article, "source_snapshots", "SourceSnapshots")
	if len(snapshots) != 1 || historyAPIString(historyAPIObject(t, snapshots[0]), "id", "ID") != "snapshot-article" {
		t.Fatalf("unexpected article source snapshots: %#v", snapshots)
	}
}

func TestGetDraftHandlerReturnsSectionRegenerations(t *testing.T) {
	setupHistoryAPIStore(t)

	request := httptest.NewRequest(http.MethodGet, "/api/drafts/draft-history", nil)
	request = mux.SetURLVars(request, map[string]string{"id": "draft-history"})
	response := httptest.NewRecorder()
	GetDraftHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	draft := historyAPINamedObject(t, decodeHistoryAPIPayload(t, response), "draft")
	if got := historyAPIString(draft, "id", "ID"); got != "draft-history" {
		t.Fatalf("draft id = %q, want draft-history: %#v", got, draft)
	}
	if got := historyAPIString(draft, "markdown", "Markdown"); got != "# History draft\n\nOriginal body." {
		t.Fatalf("draft markdown = %q, want seeded markdown: %#v", got, draft)
	}
	regenerations := historyAPIArrayFromObject(t, draft, "section_regenerations", "SectionRegenerations")
	if len(regenerations) != 1 {
		t.Fatalf("section regenerations = %d, want 1: %#v", len(regenerations), regenerations)
	}
	regeneration := historyAPIObject(t, regenerations[0])
	if got := historyAPIString(regeneration, "section_anchor", "SectionAnchor"); got != "history-anchor" {
		t.Fatalf("section anchor = %q, want history-anchor: %#v", got, regeneration)
	}
}

func TestHistoryReadHandlersReturnNotFoundForMissingIDs(t *testing.T) {
	setupHistoryAPIStore(t)

	tests := []struct {
		name    string
		target  string
		handler http.HandlerFunc
	}{
		{name: "project", target: "/api/projects/missing", handler: GetProjectHandler},
		{name: "article", target: "/api/articles/missing", handler: GetArticleHandler},
		{name: "draft", target: "/api/drafts/missing", handler: GetDraftHandler},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.target, nil)
			request = mux.SetURLVars(request, map[string]string{"id": "missing"})
			response := httptest.NewRecorder()

			tt.handler(response, request)

			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestSaveGeneratedDraftHistoryPersistsDraftAndRegeneration(t *testing.T) {
	store := setupEmptyHistoryAPIStore(t)
	persona, ok := personadomain.DefaultRegistry().Get(personadomain.IDCloudia)
	if !ok {
		t.Fatal("missing cloudia persona")
	}
	format, ok := outputformat.DefaultRegistry().Get(outputformat.IDNoteArticle)
	if !ok {
		t.Fatal("missing note format")
	}
	generatedDraft, err := articledomain.NewDraftForFormat("# Saved draft\n\nOriginal body.", format.ID)
	if err != nil {
		t.Fatalf("new generated draft: %v", err)
	}

	draftID := saveGeneratedDraftHistory(generateDraftRequest{
		StyleProfileID: "style-save",
		SessionID:      "session-save",
	}, draftapp.GenerateResult{
		Draft: generatedDraft,
		Evaluation: draftapp.StyleEvaluation{
			Passed: true,
		},
		Verification: draftapp.FinalVerification{
			Performed: true,
			Passed:    true,
			Summary:   "ok",
		},
	}, briefdomain.ArticleBrief{
		Theme:          "Saved draft",
		StyleProfileID: "style-save",
		PersonaID:      persona.ID,
		OutputFormatID: format.ID,
	}, persona, format)
	if draftID == "" {
		t.Fatal("draft history id was empty")
	}

	articleID := historyRecordID("article", "session-save")
	article, ok := store.GetArticle(articleID)
	if !ok || article.CurrentDraftID != draftID {
		t.Fatalf("saved article = %#v ok=%v, want current draft %s", article, ok, draftID)
	}
	savedDraft, ok := store.GetDraft(draftID)
	if !ok || savedDraft.ArticleID != articleID || savedDraft.Version != 1 {
		t.Fatalf("saved draft = %#v ok=%v", savedDraft, ok)
	}

	regenerationID := saveSectionRegenerationHistory(savedDraft, true, regenerateDraftSectionRequest{
		SectionAnchor: "saved",
	}, draftapp.RegenerateSectionResult{
		Section:              draftapp.MarkdownSection{Anchor: "saved", Heading: "Saved"},
		ReplacementMarkdown:  "## Saved\n\nUpdated body.",
		UpdatedDraftMarkdown: "# Saved draft\n\nUpdated body.",
	})
	if regenerationID == "" {
		t.Fatal("section regeneration history id was empty")
	}
	regenerations, err := store.ListSectionRegenerations(draftID)
	if err != nil {
		t.Fatalf("list section regenerations: %v", err)
	}
	if len(regenerations) != 1 || regenerations[0].ID != regenerationID || regenerations[0].Version != 2 {
		t.Fatalf("unexpected regenerations: %#v", regenerations)
	}
}

func setupHistoryAPIStore(t *testing.T) {
	t.Helper()
	store := setupEmptyHistoryAPIStore(t)
	seedHistoryAPIStore(t, store)
}

func setupEmptyHistoryAPIStore(t *testing.T) *sqliterepo.WorkflowStore {
	t.Helper()
	previous := workflowStore
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	store, err := sqliterepo.NewWorkflowStoreDB(db)
	if err != nil {
		_ = db.Close()
		t.Fatalf("new sqlite workflow store: %v", err)
	}
	t.Cleanup(func() {
		workflowStore = previous
		_ = store.Close()
	})
	workflowStore = store
	return store
}

func seedHistoryAPIStore(t *testing.T, store *sqliterepo.WorkflowStore) {
	t.Helper()
	now := time.Unix(1710000000, 0).UTC()
	project := sqliterepo.ProjectRecord{
		ID:        "project-history",
		Name:      "History project",
		CreatedAt: now,
		UpdatedAt: now.Add(2 * time.Minute),
		Metadata:  map[string]any{"owner": "history-test"},
	}
	if err := store.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}
	article := sqliterepo.ArticleRecord{
		ID:             "article-history",
		ProjectID:      project.ID,
		PersonaID:      personadomain.IDCloudia,
		OutputFormatID: outputformat.IDZennArticle,
		BriefSessionID: "brief-history",
		Title:          "History article",
		CreatedAt:      now,
		UpdatedAt:      now.Add(time.Minute),
	}
	if err := store.SaveArticle(article); err != nil {
		t.Fatalf("save article: %v", err)
	}
	if err := store.SaveSourceSnapshot(sqliterepo.SourceSnapshotRecord{
		ID:        "snapshot-project",
		ScopeType: "project",
		ScopeID:   project.ID,
		Selector:  sourcedomain.Ref{Kind: sourcedomain.KindZenn, Ref: "cloudia"},
		Profile: &sourcedomain.ProfileSnapshot{
			Kind:      sourcedomain.KindZenn,
			Ref:       "cloudia",
			Title:     "Cloudia",
			FetchedAt: now,
		},
		FetchedAt: now,
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("save project source snapshot: %v", err)
	}
	if err := store.SaveSourceSnapshot(sqliterepo.SourceSnapshotRecord{
		ID:        "snapshot-article",
		ScopeType: "article",
		ScopeID:   article.ID,
		Selector:  sourcedomain.Ref{Kind: sourcedomain.KindZenn, URL: "https://zenn.dev/cloudia/articles/history"},
		Article: &sourcedomain.ArticleSnapshot{
			ID:        "source-history",
			Kind:      sourcedomain.KindZenn,
			URL:       "https://zenn.dev/cloudia/articles/history",
			Title:     "Source history",
			Content:   "Source body",
			FetchedAt: now,
		},
		FetchedAt: now,
		CreatedAt: now.Add(time.Second),
	}); err != nil {
		t.Fatalf("save article source snapshot: %v", err)
	}
	draft := sqliterepo.DraftRecord{
		ID:             "draft-history",
		ArticleID:      article.ID,
		SessionID:      article.BriefSessionID,
		StyleProfileID: "style-history",
		PersonaID:      article.PersonaID,
		OutputFormatID: article.OutputFormatID,
		Version:        1,
		Markdown:       "# History draft\n\nOriginal body.",
		Evaluation: draftapp.StyleEvaluation{
			Passed: true,
		},
		Verification: draftapp.FinalVerification{
			Performed: true,
			Passed:    true,
			Summary:   "ok",
		},
		CreatedAt: now.Add(3 * time.Minute),
	}
	if err := store.SaveDraft(draft); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	if err := store.SaveSectionRegeneration(sqliterepo.SectionRegenerationRecord{
		ID:                   "regen-history",
		DraftID:              draft.ID,
		ArticleID:            article.ID,
		SectionAnchor:        "history-anchor",
		SectionHeading:       "History",
		BaseVersion:          1,
		Version:              2,
		ReplacementMarkdown:  "## History\n\nUpdated body.",
		UpdatedDraftMarkdown: "# History draft\n\nUpdated body.",
		Verification: draftapp.FinalVerification{
			Performed: true,
			Passed:    true,
			Summary:   "regenerated ok",
		},
		CreatedAt: now.Add(4 * time.Minute),
	}); err != nil {
		t.Fatalf("save section regeneration: %v", err)
	}
}

func decodeHistoryAPIPayload(t *testing.T, response *httptest.ResponseRecorder) any {
	t.Helper()
	var payload any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return payload
}

func historyAPIArray(t *testing.T, payload any, names ...string) []any {
	t.Helper()
	if array, ok := payload.([]any); ok {
		return array
	}
	object := historyAPIObject(t, payload)
	return historyAPIArrayFromObject(t, object, names...)
}

func historyAPINamedObject(t *testing.T, payload any, names ...string) map[string]any {
	t.Helper()
	object := historyAPIObject(t, payload)
	for _, name := range names {
		if value, ok := object[name]; ok {
			return historyAPIObject(t, value)
		}
	}
	return object
}

func historyAPIArrayFromObject(t *testing.T, object map[string]any, names ...string) []any {
	t.Helper()
	for _, name := range names {
		if value, ok := object[name]; ok {
			array, ok := value.([]any)
			if !ok {
				t.Fatalf("%s is %T, want array: %#v", name, value, object)
			}
			return array
		}
	}
	t.Fatalf("none of %v found in response object: %#v", names, object)
	return nil
}

func historyAPIObject(t *testing.T, value any) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value is %T, want object: %#v", value, value)
	}
	return object
}

func historyAPIString(object map[string]any, names ...string) string {
	for _, name := range names {
		if value, ok := object[name]; ok {
			if text, ok := value.(string); ok {
				return text
			}
		}
	}
	return ""
}
