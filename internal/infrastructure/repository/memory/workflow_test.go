package memory

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/teradakousuke/note_maker/internal/application/authorstyle"
	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

func TestPersistentWorkflowStoreRestoresDraftInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow_store.json")
	store, err := NewPersistentWorkflowStore(path)
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
	editedBrief := brief
	editedBrief.Theme = "Edited local workflow brief"
	if err := store.SaveBrief(session.ID, editedBrief); err != nil {
		t.Fatalf("save edited brief: %v", err)
	}

	reopened, err := NewPersistentWorkflowStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	profile, guide, ok := reopened.GetProfileAndGuide(result.Profile.ID)
	if !ok {
		t.Fatal("expected profile and guide after reopen")
	}
	if profile.ID != result.Profile.ID || guide.ID != result.Guide.ID {
		t.Fatalf("restored wrong style assets: profile=%s guide=%s", profile.ID, guide.ID)
	}
	restoredSession, ok := reopened.GetSession(session.ID)
	if !ok {
		t.Fatal("expected session after reopen")
	}
	if len(restoredSession.Answers) != len(session.Answers) {
		t.Fatalf("answers = %d, want %d", len(restoredSession.Answers), len(session.Answers))
	}
	restoredBrief, ok := reopened.GetBrief(session.ID)
	if !ok {
		t.Fatal("expected brief after reopen")
	}
	if restoredBrief.Theme != editedBrief.Theme || restoredBrief.PersonalContext == "" || len(restoredBrief.DeepDives) != 1 || len(restoredBrief.CustomAnswers) != 1 {
		t.Fatalf("brief did not preserve generation context: %#v", restoredBrief)
	}
	versions, err := reopened.ListBriefVersions(session.ID)
	if err != nil {
		t.Fatalf("list brief versions: %v", err)
	}
	if len(versions) != 2 || versions[0].Brief.Theme == editedBrief.Theme || versions[1].Brief.Theme != editedBrief.Theme {
		t.Fatalf("unexpected brief versions: %#v", versions)
	}
}

func TestPersistentWorkflowStoreRestoresCustomPersonas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow_store.json")
	store, err := NewPersistentWorkflowStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	persona := testCustomPersona()
	if err := store.SavePersona(persona); err != nil {
		t.Fatalf("save persona: %v", err)
	}

	reopened, err := NewPersistentWorkflowStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	restored, ok := reopened.GetPersona(persona.ID)
	if !ok {
		t.Fatal("expected persona after reopen")
	}
	if restored.DisplayName != persona.DisplayName || restored.VoiceNotes.Tone != persona.VoiceNotes.Tone {
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

func TestWorkflowStoreDeletesOnlyUnreferencedCustomPersonas(t *testing.T) {
	store := NewWorkflowStore()
	persona := testCustomPersona()
	if err := store.SavePersona(persona); err != nil {
		t.Fatalf("save persona: %v", err)
	}
	if err := store.DeletePersona(persona.ID); err != nil {
		t.Fatalf("delete unreferenced persona: %v", err)
	}
	if _, ok := store.GetPersona(persona.ID); ok {
		t.Fatal("persona should be deleted")
	}
	if err := store.DeletePersona(persona.ID); err != personadomain.ErrPersonaNotFound {
		t.Fatalf("delete missing persona err = %v, want ErrPersonaNotFound", err)
	}

	if err := store.SavePersona(persona); err != nil {
		t.Fatalf("resave persona: %v", err)
	}
	session, err := briefdomain.NewArticleBriefSessionWithOptions("session_custom", "profile_custom", persona.ID, outputformat.IDNoteArticle, "", briefdomain.FixedQuestions())
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if err := store.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := store.DeletePersona(persona.ID); err != personadomain.ErrPersonaReferenced {
		t.Fatalf("delete referenced persona err = %v, want ErrPersonaReferenced", err)
	}
}

func testAnalyzeResult(t *testing.T) authorstyle.AnalyzeResult {
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
	return authorstyle.AnalyzeResult{
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
