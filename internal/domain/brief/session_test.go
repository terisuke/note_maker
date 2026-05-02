package brief

import (
	"reflect"
	"strings"
	"testing"

	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	"github.com/teradakousuke/note_maker/internal/domain/persona"
)

func TestFixedQuestionsAreDeterministic(t *testing.T) {
	questions := FixedQuestions()
	wantIDs := []string{
		QuestionIDTheme,
		QuestionIDOpeningEpisode,
		QuestionIDReader,
		QuestionIDExpectedReaderAction,
		QuestionIDMustInclude,
		QuestionIDPersonalContext,
		QuestionIDExclusions,
		QuestionIDTargetLengthStructure,
		QuestionIDToneStance,
	}
	wantText := []string{
		"記事の中心テーマは何ですか？",
		"記事の導入に置く具体的な体験や場面は何ですか？",
		"この記事を届けたい読者は誰ですか？",
		"読後に読者へどんな変化や行動を起こしてほしいですか？",
		"記事に必ず含める論点、事実、手順は何ですか？",
		"著者本人の経験、肩書き、失敗、価値観など、記事に入れるべき属人的な文脈は何ですか？",
		"記事に含めないこと、避けたい表現、断言しないことは何ですか？",
		"目標文字数と記事構成を指定してください。例: 3000字前後、導入・背景・実装・検証・提案・結論。",
		"記事のトーンや立場はどうしますか？内省、技術解説、実用、物語性の比重も指定してください。",
	}
	if len(questions) != len(wantIDs) {
		t.Fatalf("question count = %d, want %d", len(questions), len(wantIDs))
	}
	for i, question := range questions {
		if question.ID != wantIDs[i] {
			t.Fatalf("question %d id = %q, want %q", i, question.ID, wantIDs[i])
		}
		if question.Text != wantText[i] {
			t.Fatalf("question %d text = %q, want %q", i, question.Text, wantText[i])
		}
		if question.FlowType != QuestionFlowMain {
			t.Fatalf("question %d flow = %q, want %q", i, question.FlowType, QuestionFlowMain)
		}
	}
}

func TestComposeFixedQuestionsCoversPersonasAndFormats(t *testing.T) {
	personas := []string{persona.IDTerisuke, persona.IDCloudia}
	formats := []string{
		outputformat.IDNoteArticle,
		outputformat.IDMarkdownBlog,
		outputformat.IDZennArticle,
		outputformat.IDQiitaArticle,
		outputformat.IDHomepageSection,
	}
	for _, personaID := range personas {
		for _, formatID := range formats {
			t.Run(personaID+"_"+formatID, func(t *testing.T) {
				questions := ComposeFixedQuestions(personaID, formatID)
				if len(questions) < len(FixedQuestions()) {
					t.Fatalf("question count = %d, want at least %d", len(questions), len(FixedQuestions()))
				}
				assertUniqueQuestionIDs(t, questions)
				if personaID == persona.IDTerisuke && formatID == outputformat.IDNoteArticle {
					if !reflect.DeepEqual(questions, FixedQuestions()) {
						t.Fatalf("terisuke note_article template changed:\ngot  %#v\nwant %#v", questions, FixedQuestions())
					}
					return
				}
				switch formatID {
				case outputformat.IDNoteArticle:
					assertQuestionPresent(t, questions, QuestionIDStoryArc)
				case outputformat.IDMarkdownBlog, outputformat.IDZennArticle, outputformat.IDQiitaArticle:
					assertQuestionPresent(t, questions, QuestionIDTargetStack)
				case outputformat.IDHomepageSection:
					assertQuestionPresent(t, questions, QuestionIDHomepageCTA)
				}
				if personaID == persona.IDCloudia {
					assertQuestionPresent(t, questions, QuestionIDCloudiaViewpoint)
				}
				if personaID == persona.IDCloudia && formatID == outputformat.IDZennArticle {
					assertQuestionPresent(t, questions, QuestionIDTargetStack)
				}
			})
		}
	}
}

func TestSelectDeepDiveTargetsUsesHighValueDeterministicOrder(t *testing.T) {
	session := answeredFixedSession(t, map[string]string{
		QuestionIDOpeningEpisode:        "The article opens on a late night debugging session.",
		QuestionIDMustInclude:           "Include local LLM latency numbers.",
		QuestionIDPersonalContext:       "Include the author's background as a musician and engineer.",
		QuestionIDExpectedReaderAction:  "Try the workflow on one small article.",
		QuestionIDToneStance:            "Practical but reflective.",
		QuestionIDTargetLengthStructure: "1200 words",
	})

	targets := session.SelectDeepDiveTargets()
	want := []string{
		QuestionIDOpeningEpisode,
		QuestionIDMustInclude,
		QuestionIDPersonalContext,
		QuestionIDExpectedReaderAction,
		QuestionIDToneStance,
	}
	if len(targets) != len(want) {
		t.Fatalf("target count = %d, want %d", len(targets), len(want))
	}
	for i, target := range targets {
		if target.ID != want[i] {
			t.Fatalf("target %d = %q, want %q", i, target.ID, want[i])
		}
	}
}

func TestFollowUpCountLimits(t *testing.T) {
	session := answeredFixedSession(t, nil)

	var asked []ArticleQuestion
	for i := 0; i < MaxTotalFollowUps; i++ {
		question, ok := session.CurrentQuestion()
		if !ok {
			t.Fatalf("expected follow-up %d", i+1)
		}
		if question.FlowType != QuestionFlowDeepDiveFollowUp {
			t.Fatalf("flow = %q, want %q", question.FlowType, QuestionFlowDeepDiveFollowUp)
		}
		asked = append(asked, question)
		if _, err := session.RecordAnswer("A concrete detail for the follow-up."); err != nil {
			t.Fatalf("record follow-up %d: %v", i+1, err)
		}
		if session.DeepDiveAnswerCountForTarget(question.TargetQuestionID) > MaxFollowUpsPerTarget {
			t.Fatalf("target %q exceeded per-target cap", question.TargetQuestionID)
		}
	}
	if session.DeepDiveAnswerCount() != MaxTotalFollowUps {
		t.Fatalf("deep-dive count = %d, want %d", session.DeepDiveAnswerCount(), MaxTotalFollowUps)
	}
	if _, ok := session.CurrentQuestion(); ok {
		t.Fatal("expected no question after total follow-up cap")
	}
	if asked[0].TargetQuestionID != QuestionIDOpeningEpisode || asked[1].TargetQuestionID != QuestionIDOpeningEpisode {
		t.Fatalf("first target was not capped after two follow-ups: %#v", asked[:2])
	}
	if asked[2].TargetQuestionID != QuestionIDMustInclude || asked[3].TargetQuestionID != QuestionIDMustInclude {
		t.Fatalf("second target did not receive remaining follow-ups: %#v", asked[2:])
	}
}

func TestCompletionRules(t *testing.T) {
	session, err := NewArticleBriefSession("session-1", "style-1")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if session.CanComplete() {
		t.Fatal("new session should not be complete")
	}

	session = answeredFixedSession(t, nil)
	if session.CanComplete() {
		t.Fatal("session should require at least one deep dive when high-value targets exist")
	}

	question, ok := session.CurrentQuestion()
	if !ok {
		t.Fatal("expected deep-dive question")
	}
	if _, err := session.RecordAnswer("The opening emotion is a mix of doubt and relief."); err != nil {
		t.Fatalf("record deep dive: %v", err)
	}
	if question.TargetQuestionID != QuestionIDOpeningEpisode {
		t.Fatalf("deep-dive target = %q, want %q", question.TargetQuestionID, QuestionIDOpeningEpisode)
	}
	if !session.CanComplete() {
		t.Fatal("session should complete after required fields and one deep dive")
	}

	skipped := answeredFixedSession(t, nil)
	skipped.MarkDeepDiveSkipped()
	if !skipped.CanComplete() {
		t.Fatal("explicit deep-dive skip should allow completion")
	}
}

func TestAssembleBriefIncludesFixedAndDeepDiveAnswers(t *testing.T) {
	session := answeredFixedSession(t, map[string]string{
		QuestionIDTargetLengthStructure: "3000字前後、導入・背景・実装・検証・結論",
		QuestionIDExclusions:            "Do not mention hosted SaaS.",
	})
	if _, err := session.RecordAnswer("Show the screen going from timeout to a usable draft."); err != nil {
		t.Fatalf("record deep dive: %v", err)
	}

	articleBrief, err := session.Complete()
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if articleBrief.StyleProfileID != "style-1" {
		t.Fatalf("style profile id = %q", articleBrief.StyleProfileID)
	}
	if articleBrief.Theme != "Local article generation with a small deterministic workflow." {
		t.Fatalf("theme = %q", articleBrief.Theme)
	}
	if articleBrief.TargetLengthStructure != "3000字前後、導入・背景・実装・検証・結論" {
		t.Fatalf("target length = %q", articleBrief.TargetLengthStructure)
	}
	if articleBrief.Exclusions != "Do not mention hosted SaaS." {
		t.Fatalf("exclusions = %q", articleBrief.Exclusions)
	}
	if articleBrief.PersonalContext == "" {
		t.Fatal("personal context should be assembled")
	}
	if len(articleBrief.DeepDives) != 1 {
		t.Fatalf("deep dives = %d, want 1", len(articleBrief.DeepDives))
	}
	if articleBrief.DeepDives[0].TargetQuestionID != QuestionIDOpeningEpisode {
		t.Fatalf("deep-dive target = %q", articleBrief.DeepDives[0].TargetQuestionID)
	}
}

func TestForkWithEditedAnswerKeepsOriginalAndTruncatesFollowingAnswers(t *testing.T) {
	session, err := NewArticleBriefSession("session-1", "style-1")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if _, err := session.RecordAnswer("Initial theme"); err != nil {
		t.Fatalf("answer theme: %v", err)
	}
	if _, err := session.RecordAnswer("Initial opening"); err != nil {
		t.Fatalf("answer opening: %v", err)
	}
	if _, err := session.RecordAnswer("Initial reader"); err != nil {
		t.Fatalf("answer reader: %v", err)
	}

	fork, err := session.ForkWithEditedAnswer("session-2", QuestionIDOpeningEpisode, "Edited opening")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if fork.ID != "session-2" {
		t.Fatalf("fork id = %q", fork.ID)
	}
	if fork.ParentSessionID != "session-1" {
		t.Fatalf("parent session id = %q", fork.ParentSessionID)
	}
	if len(fork.Answers) != 2 {
		t.Fatalf("fork answers = %d, want 2", len(fork.Answers))
	}
	if fork.Answers[0].QuestionID != QuestionIDTheme || fork.Answers[0].Content != "Initial theme" {
		t.Fatalf("first fork answer = %#v", fork.Answers[0])
	}
	if fork.Answers[1].QuestionID != QuestionIDOpeningEpisode || fork.Answers[1].Content != "Edited opening" {
		t.Fatalf("edited fork answer = %#v", fork.Answers[1])
	}
	if session.Answers[1].Content != "Initial opening" || len(session.Answers) != 3 {
		t.Fatalf("original session was mutated: %#v", session.Answers)
	}
	next, ok := fork.CurrentQuestion()
	if !ok {
		t.Fatal("expected next question after fork")
	}
	if next.ID != QuestionIDReader {
		t.Fatalf("next question = %q, want %q", next.ID, QuestionIDReader)
	}
}

func TestGeneratedFollowUpQuestionRulesRejectBinaryQuestions(t *testing.T) {
	disallowed := []string{
		"Do you want the article to be practical?",
		"Should it be technical or narrative?",
		"Is the reader a beginner?",
	}
	for _, text := range disallowed {
		if IsAllowedFollowUpQuestion(text) {
			t.Fatalf("expected %q to be rejected", text)
		}
	}
	if !IsAllowedFollowUpQuestion("What concrete scene should make that point memorable?") {
		t.Fatal("expected open-ended follow-up to be allowed")
	}
}

func TestFallbackFollowUpTextIncludesContextualPrefix(t *testing.T) {
	target := FixedQuestions()[1]
	answer := BriefAnswer{
		QuestionID: target.ID,
		Content:    "以前の生成記事を読んだとき、自分の切実さが抜け落ちていると感じた",
		FlowType:   QuestionFlowMain,
	}
	got := FallbackFollowUpText(target, answer, 1)
	if !strings.HasPrefix(got, "「以前の生成記事を読んだとき、自分の切実さが抜け落ちていると感じた」というご回答を踏まえて、") {
		t.Fatalf("fallback missing contextual prefix: %q", got)
	}
	if !IsAllowedFollowUpQuestion(got) {
		t.Fatalf("contextual fallback should be allowed: %q", got)
	}
}

func TestContextualFollowUpValidationStillRejectsBinaryQuestion(t *testing.T) {
	text := "「AI or human の迷いがある」というご回答を踏まえて、Do you want the article to be practical?"
	if IsAllowedFollowUpQuestion(text) {
		t.Fatalf("expected contextual binary question to be rejected: %q", text)
	}
	allowed := "「AI or human の迷いがある」というご回答を踏まえて、どの場面からその迷いを具体的に説明しますか？"
	if !IsAllowedFollowUpQuestion(allowed) {
		t.Fatalf("expected contextual open question to be allowed: %q", allowed)
	}
}

func answeredFixedSession(t *testing.T, overrides map[string]string) ArticleBriefSession {
	t.Helper()
	session, err := NewArticleBriefSession("session-1", "style-1")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	answers := map[string]string{
		QuestionIDTheme:                 "Local article generation with a small deterministic workflow.",
		QuestionIDOpeningEpisode:        "Open with a failed local LLM run that timed out while drafting.",
		QuestionIDReader:                "Solo developers who write note.com articles with local tools.",
		QuestionIDExpectedReaderAction:  "They should try a three-phase workflow before drafting.",
		QuestionIDMustInclude:           "Mention style analysis, brief interviews, and final draft checks.",
		QuestionIDPersonalContext:       "Use the author's background as a musician, engineer, and public speaker.",
		QuestionIDExclusions:            "Avoid cloud-only assumptions.",
		QuestionIDTargetLengthStructure: "3000字前後 with six sections.",
		QuestionIDToneStance:            "Practical and introspective.",
	}
	for key, value := range overrides {
		answers[key] = value
	}
	for _, question := range FixedQuestions() {
		if _, err := session.RecordAnswer(answers[question.ID]); err != nil {
			t.Fatalf("answer %s: %v", question.ID, err)
		}
	}
	return session
}

func assertQuestionPresent(t *testing.T, questions []ArticleQuestion, id string) {
	t.Helper()
	for _, question := range questions {
		if question.ID == id {
			return
		}
	}
	t.Fatalf("question %q was not present in %#v", id, questions)
}

func assertUniqueQuestionIDs(t *testing.T, questions []ArticleQuestion) {
	t.Helper()
	seen := map[string]bool{}
	for _, question := range questions {
		if seen[question.ID] {
			t.Fatalf("duplicate question id %q in %#v", question.ID, questions)
		}
		seen[question.ID] = true
	}
}
