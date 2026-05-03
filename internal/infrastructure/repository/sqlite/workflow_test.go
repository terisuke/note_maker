package sqlite

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	authorstyleapp "github.com/teradakousuke/note_maker/internal/application/authorstyle"
	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
	sourcedomain "github.com/teradakousuke/note_maker/internal/domain/source"
)

func TestWorkflowStoreRestoresDraftInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note_maker.db")
	store, err := NewWorkflowStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	result := testAnalyzeResult(t)
	if err := store.SaveAuthorStyle(result); err != nil {
		t.Fatalf("save author style: %v", err)
	}
	session := testCompletedSession(t, result.Profile.ID)
	if err := store.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	brief := session.AssembleBrief()
	if err := store.SaveBrief(session.ID, brief); err != nil {
		t.Fatalf("save brief: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := NewWorkflowStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	for _, id := range []string{result.ID, result.Profile.ID, result.Guide.ID} {
		profile, guide, ok := reopened.GetProfileAndGuide(id)
		if !ok {
			t.Fatalf("expected profile and guide for id %q after reopen", id)
		}
		if profile.ID != result.Profile.ID || guide.ID != result.Guide.ID {
			t.Fatalf("restored wrong style assets: profile=%s guide=%s", profile.ID, guide.ID)
		}
	}
	restoredSession, ok := reopened.GetSession(session.ID)
	if !ok {
		t.Fatal("expected session after reopen")
	}
	if restoredSession.PersonaID != personadomain.IDTerisuke || restoredSession.OutputFormatID != outputformat.IDNoteArticle {
		t.Fatalf("session mode was not restored: %#v", restoredSession)
	}
	if len(restoredSession.Answers) != len(session.Answers) {
		t.Fatalf("answers = %d, want %d", len(restoredSession.Answers), len(session.Answers))
	}
	restoredBrief, ok := reopened.GetBrief(session.ID)
	if !ok {
		t.Fatal("expected brief after reopen")
	}
	if restoredBrief.PersonalContext == "" || len(restoredBrief.DeepDives) != 1 || len(restoredBrief.CustomAnswers) != 1 {
		t.Fatalf("brief did not preserve generation context: %#v", restoredBrief)
	}
}

func TestWorkflowStoreRestoresCustomPersonas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note_maker.db")
	store, err := NewWorkflowStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	persona := testCustomPersona()
	if err := store.SavePersona(persona); err != nil {
		t.Fatalf("save persona: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := NewWorkflowStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	restored, ok := reopened.GetPersona(persona.ID)
	if !ok {
		t.Fatal("expected persona after reopen")
	}
	if restored.DisplayName != persona.DisplayName || restored.DefaultFormat != persona.DefaultFormat {
		t.Fatalf("unexpected restored persona: %#v", restored)
	}
	listed, err := reopened.ListPersonas()
	if err != nil {
		t.Fatalf("list personas: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != persona.ID {
		t.Fatalf("unexpected persona list: %#v", listed)
	}
}

func TestWorkflowStoreAppliesSchemaMigrations(t *testing.T) {
	store, err := NewWorkflowStore(filepath.Join(t.TempDir(), "note_maker.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var migrationCount int
	if err := store.DB().QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = 1`).Scan(&migrationCount); err != nil {
		t.Fatalf("query schema migrations: %v", err)
	}
	if migrationCount != 1 {
		t.Fatalf("migration count = %d, want 1", migrationCount)
	}
	for _, table := range []string{"projects", "articles", "brief_sessions", "brief_answers", "briefs", "custom_personas", "drafts", "section_regenerations", "source_selector_snapshots"} {
		var name string
		if err := store.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}
}

func TestWorkflowStorePersistsHistoryRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note_maker.db")
	store, err := NewWorkflowStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	now := time.Unix(1710000000, 0).UTC()

	project := ProjectRecord{
		ID:        "project_media_matrix",
		Name:      "Media matrix",
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  map[string]any{"owner": "scenario"},
	}
	if err := store.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}
	article := ArticleRecord{
		ID:             "article_zenn",
		ProjectID:      project.ID,
		PersonaID:      personadomain.IDCloudia,
		OutputFormatID: outputformat.IDZennArticle,
		BriefSessionID: "brief_zenn",
		Title:          "SQLite-backed history",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := store.SaveArticle(article); err != nil {
		t.Fatalf("save article: %v", err)
	}
	sourceArticle := sourcedomain.ArticleSnapshot{
		ID:        "source-1",
		Kind:      sourcedomain.KindZenn,
		URL:       "https://zenn.dev/cloudia/articles/sqlite-history",
		Title:     "SQLite history",
		Content:   "source body",
		FetchedAt: now,
	}
	if err := store.SaveSourceSnapshot(SourceSnapshotRecord{
		ID:        "snapshot-1",
		ScopeType: "article",
		ScopeID:   article.ID,
		Selector: sourcedomain.Ref{
			Kind: sourcedomain.KindZenn,
			Ref:  "cloudia",
		},
		Article:   &sourceArticle,
		FetchedAt: now,
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("save source snapshot: %v", err)
	}
	draftRecord := DraftRecord{
		ID:             "draft-1",
		ArticleID:      article.ID,
		SessionID:      article.BriefSessionID,
		StyleProfileID: "profile-1",
		PersonaID:      article.PersonaID,
		OutputFormatID: article.OutputFormatID,
		Version:        1,
		Markdown:       "---\ntitle: \"SQLite\"\nemoji: \"🧪\"\ntype: \"tech\"\ntopics: [\"go\"]\npublished: false\n---\n\n## 実装\n\n本文です。",
		Evaluation: draftapp.StyleEvaluation{
			Passed: true,
		},
		Verification: draftapp.FinalVerification{
			Performed: true,
			Passed:    true,
			Summary:   "ok",
		},
		CreatedAt: now,
	}
	if err := store.SaveDraft(draftRecord); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	if err := store.SaveSectionRegeneration(SectionRegenerationRecord{
		ID:                   "regen-1",
		DraftID:              draftRecord.ID,
		ArticleID:            article.ID,
		SectionAnchor:        "implementation",
		SectionHeading:       "実装",
		BaseVersion:          1,
		Version:              2,
		ReplacementMarkdown:  "## 実装\n\n更新後です。",
		UpdatedDraftMarkdown: "---\ntitle: \"SQLite\"\nemoji: \"🧪\"\ntype: \"tech\"\ntopics: [\"go\"]\npublished: false\n---\n\n## 実装\n\n更新後です。",
		Verification: draftapp.FinalVerification{
			Performed: true,
			Passed:    true,
			Summary:   "regenerated ok",
		},
		CreatedAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("save section regeneration: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := NewWorkflowStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	restoredProject, ok := reopened.GetProject(project.ID)
	if !ok || restoredProject.Name != project.Name || restoredProject.Metadata["owner"] != "scenario" {
		t.Fatalf("unexpected restored project: %#v ok=%v", restoredProject, ok)
	}
	restoredArticle, ok := reopened.GetArticle(article.ID)
	if !ok || restoredArticle.CurrentDraftID != draftRecord.ID {
		t.Fatalf("unexpected restored article: %#v ok=%v", restoredArticle, ok)
	}
	snapshots, err := reopened.ListSourceSnapshots("article", article.ID)
	if err != nil {
		t.Fatalf("list source snapshots: %v", err)
	}
	if len(snapshots) != 1 || snapshots[0].ContentHash == "" || snapshots[0].Article.Content != sourceArticle.Content {
		t.Fatalf("unexpected snapshots: %#v", snapshots)
	}
	drafts, err := reopened.ListDrafts(article.ID)
	if err != nil {
		t.Fatalf("list drafts: %v", err)
	}
	if len(drafts) != 1 || drafts[0].ContentHash == "" || !drafts[0].Verification.Passed || drafts[0].QuestionTemplateVersion == "" {
		t.Fatalf("unexpected drafts: %#v", drafts)
	}
	regenerations, err := reopened.ListSectionRegenerations(draftRecord.ID)
	if err != nil {
		t.Fatalf("list regenerations: %v", err)
	}
	if len(regenerations) != 1 || regenerations[0].Version != 2 || !strings.Contains(regenerations[0].UpdatedDraftMarkdown, "更新後") {
		t.Fatalf("unexpected regenerations: %#v", regenerations)
	}
}

func testAnalyzeResult(t *testing.T) authorstyleapp.AnalyzeResult {
	t.Helper()
	fetchedAt := time.Unix(1700000000, 0).UTC()
	article := articledomain.Article{
		URL:     "https://note.com/tera/n/n111",
		Title:   "AIと音楽",
		Content: "僕はAIと音楽の違和感を言語化する。\n\n「これは大事だ」と思った。",
	}
	source := authordomain.AuthorSource{
		Username:  "tera",
		Articles:  authordomain.SourceArticlesFromArticles([]articledomain.Article{article}, fetchedAt),
		FetchedAt: fetchedAt,
	}
	profile, err := authordomain.BuildAuthorStyleProfile(source, []articledomain.Article{article})
	if err != nil {
		t.Fatalf("build profile: %v", err)
	}
	guide, err := authordomain.BuildWritingStyleGuide(profile)
	if err != nil {
		t.Fatalf("build guide: %v", err)
	}
	return authorstyleapp.AnalyzeResult{
		ID:           "asr_test",
		Source:       source,
		Profile:      profile,
		Guide:        guide,
		ArticleCount: 1,
		CreatedAt:    fetchedAt,
	}
}

func testCustomPersona() personadomain.Persona {
	return personadomain.Persona{
		ID:            "custom_writer",
		DisplayName:   "Custom Writer",
		Description:   "A locally authored test persona.",
		DefaultFormat: outputformat.IDNoteArticle,
		Sources: []personadomain.AuthorSource{
			{Kind: "note", Ref: "custom_writer"},
		},
		VoiceNotes: personadomain.VoiceNotes{
			FirstPerson:   []string{"私"},
			Tone:          "Calm, direct, and specific.",
			TitlePatterns: []string{"How I use local workflows"},
			AntiPatterns:  []string{"empty claims"},
		},
	}
}

func testCompletedSession(t *testing.T, profileID string) briefdomain.ArticleBriefSession {
	t.Helper()
	questions := append(briefdomain.FixedQuestions(), briefdomain.ArticleQuestion{
		ID:          "custom_origin",
		Text:        "どの原体験を入れますか？",
		FlowType:    briefdomain.QuestionFlowMain,
		Required:    false,
		TargetField: "custom",
	})
	session, err := briefdomain.NewArticleBriefSessionWithQuestions("brief_test", profileID, questions)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	answers := map[string]string{
		briefdomain.QuestionIDTheme:                 "ローカルLLMで思想を言語化する",
		briefdomain.QuestionIDOpeningEpisode:        "生成文を読んで自分の切実さが抜けていた",
		briefdomain.QuestionIDReader:                "AIで発信したい個人開発者",
		briefdomain.QuestionIDExpectedReaderAction:  "AIに取材させる視点を持つ",
		briefdomain.QuestionIDMustInclude:           "Note API、文体ガイド、一問一答、深掘り",
		briefdomain.QuestionIDPersonalContext:       "音楽家、エンジニア、起業、LT登壇の経験",
		briefdomain.QuestionIDExclusions:            "根拠のない性能断言",
		briefdomain.QuestionIDTargetLengthStructure: "3000字前後、最低2800字",
		briefdomain.QuestionIDToneStance:            "内省と技術検証を両立する",
		"custom_origin":                             "音楽の練習で身体化した感覚",
	}
	for _, question := range questions {
		if _, err := session.RecordAnswer(answers[question.ID]); err != nil {
			t.Fatalf("answer %s: %v", question.ID, err)
		}
	}
	if _, err := session.RecordAnswer("自分の言葉を失う怖さを最初に見せる"); err != nil {
		t.Fatalf("answer deep dive: %v", err)
	}
	if _, err := session.Complete(); err != nil {
		t.Fatalf("complete: %v", err)
	}
	return session
}
