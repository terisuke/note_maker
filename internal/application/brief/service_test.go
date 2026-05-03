package brief

import (
	"context"
	"errors"
	"testing"

	domain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	"github.com/teradakousuke/note_maker/internal/domain/persona"
)

func TestInterviewServiceStartsAndCompletesWorkflow(t *testing.T) {
	service := NewInterviewService(nil)

	result, err := service.StartSession(StartSessionInput{
		SessionID:      "session-1",
		StyleProfileID: "style-1",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if result.NextQuestion == nil || result.NextQuestion.ID != domain.QuestionIDTheme {
		t.Fatalf("first question = %#v", result.NextQuestion)
	}

	session := result.Session
	for _, answer := range fixedAnswers() {
		result, err = service.Answer(context.Background(), session, answer)
		if err != nil {
			t.Fatalf("answer fixed question: %v", err)
		}
		session = result.Session
	}
	if result.NextQuestion == nil {
		t.Fatal("expected first deep-dive question")
	}
	if result.NextQuestion.FlowType != domain.QuestionFlowDeepDiveFollowUp {
		t.Fatalf("flow = %q, want %q", result.NextQuestion.FlowType, domain.QuestionFlowDeepDiveFollowUp)
	}
	if result.NextQuestion.TargetQuestionID != domain.QuestionIDOpeningEpisode {
		t.Fatalf("deep-dive target = %q", result.NextQuestion.TargetQuestionID)
	}

	for !result.Completed {
		result, err = service.Answer(context.Background(), session, "This follow-up answer adds concrete detail.")
		if err != nil {
			t.Fatalf("answer deep dive: %v", err)
		}
		session = result.Session
	}
	if result.Brief == nil {
		t.Fatal("expected completed brief")
	}
	if result.Brief.Theme != fixedAnswers()[0] {
		t.Fatalf("brief theme = %q", result.Brief.Theme)
	}
	if len(result.Brief.DeepDives) != domain.MaxTotalFollowUps {
		t.Fatalf("deep dives = %d, want %d", len(result.Brief.DeepDives), domain.MaxTotalFollowUps)
	}
}

func TestInterviewServiceUsesFallbackForInvalidGeneratedFollowUp(t *testing.T) {
	service := NewInterviewService(staticFollowUpGenerator{
		text: "Should it be practical or technical?",
	})
	result, err := service.StartSession(StartSessionInput{
		SessionID:      "session-1",
		StyleProfileID: "style-1",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	session := result.Session
	for _, answer := range fixedAnswers() {
		result, err = service.Answer(context.Background(), session, answer)
		if err != nil {
			t.Fatalf("answer fixed question: %v", err)
		}
		session = result.Session
	}
	if result.NextQuestion == nil {
		t.Fatal("expected deep-dive question")
	}
	if result.NextQuestion.Text == "Should it be practical or technical?" {
		t.Fatal("binary generated question should have been replaced by fallback")
	}
	if !domain.IsAllowedFollowUpQuestion(result.NextQuestion.Text) {
		t.Fatalf("fallback question was not allowed: %q", result.NextQuestion.Text)
	}
}

func TestInterviewServiceUsesFallbackWhenGeneratorFails(t *testing.T) {
	service := NewInterviewService(staticFollowUpGenerator{err: errors.New("llm unavailable")})
	result, err := service.StartSession(StartSessionInput{
		SessionID:      "session-1",
		StyleProfileID: "style-1",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	session := result.Session
	for _, answer := range fixedAnswers() {
		result, err = service.Answer(context.Background(), session, answer)
		if err != nil {
			t.Fatalf("answer fixed question: %v", err)
		}
		session = result.Session
	}
	if result.NextQuestion == nil || result.NextQuestion.Text == "" {
		t.Fatal("expected fallback question")
	}
}

func TestInterviewServiceForksEditedAnswerAndReturnsNextQuestion(t *testing.T) {
	service := NewInterviewService(nil)
	result, err := service.StartSession(StartSessionInput{
		SessionID:      "session-1",
		StyleProfileID: "style-1",
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	session := result.Session
	for _, answer := range fixedAnswers()[:5] {
		result, err = service.Answer(context.Background(), session, answer)
		if err != nil {
			t.Fatalf("answer fixed question: %v", err)
		}
		session = result.Session
	}

	result, err = service.ForkAnswer(context.Background(), session, "session-2", domain.QuestionIDOpeningEpisode, "Edited opening")
	if err != nil {
		t.Fatalf("fork answer: %v", err)
	}
	if result.Session.ID != "session-2" {
		t.Fatalf("fork id = %q", result.Session.ID)
	}
	if result.Session.ParentSessionID != "session-1" {
		t.Fatalf("parent session id = %q", result.Session.ParentSessionID)
	}
	if len(result.Session.Answers) != 2 {
		t.Fatalf("answers = %d, want 2", len(result.Session.Answers))
	}
	if result.NextQuestion == nil || result.NextQuestion.ID != domain.QuestionIDReader {
		t.Fatalf("next question = %#v", result.NextQuestion)
	}
	if session.Answers[1].Content != fixedAnswers()[1] {
		t.Fatalf("original session was mutated: %#v", session.Answers[1])
	}
}

func TestInterviewServiceAppendsCustomQuestionsAfterComposedTemplate(t *testing.T) {
	service := NewInterviewService(nil)
	result, err := service.StartSession(StartSessionInput{
		SessionID:      "session-custom",
		StyleProfileID: "style-1",
		PersonaID:      persona.IDCloudia,
		OutputFormatID: outputformat.IDZennArticle,
		Questions: []domain.ArticleQuestion{
			{
				ID:          "custom_reference",
				Text:        "参考リンクとして必ず確認するURLは何ですか？",
				FlowType:    domain.QuestionFlowMain,
				TargetField: "custom",
			},
		},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	template := domain.ComposeFixedQuestions(persona.IDCloudia, outputformat.IDZennArticle)
	if len(result.Session.Questions) != len(template)+1 {
		t.Fatalf("question count = %d, want %d", len(result.Session.Questions), len(template)+1)
	}
	if result.Session.Questions[len(template)-1].ID != domain.QuestionIDCloudiaViewpoint {
		t.Fatalf("last template question = %#v", result.Session.Questions[len(template)-1])
	}
	if result.Session.Questions[len(result.Session.Questions)-1].ID != "custom_reference" {
		t.Fatalf("custom question was not appended last: %#v", result.Session.Questions)
	}
}

func TestInterviewServiceUsesPersonaDefaultFormatForTemplate(t *testing.T) {
	service := NewInterviewService(nil)
	result, err := service.StartSession(StartSessionInput{
		SessionID:      "session-cloudia-default",
		StyleProfileID: "style-1",
		PersonaID:      persona.IDCloudia,
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	if result.Session.OutputFormatID != outputformat.IDZennArticle {
		t.Fatalf("output format = %q, want %q", result.Session.OutputFormatID, outputformat.IDZennArticle)
	}
	if !sessionHasQuestion(result.Session.Questions, domain.QuestionIDTargetStack) {
		t.Fatalf("cloudia default template missing target_stack: %#v", result.Session.Questions)
	}
}

func fixedAnswers() []string {
	return []string{
		"Local article generation with a small deterministic workflow.",
		"Open with a failed local LLM run that timed out while drafting.",
		"Solo developers who write note.com articles with local tools.",
		"They are unsure how to turn rough technical notes into a readable article.",
		"They should try a three-phase workflow before drafting.",
		"The smallest useful workflow is style analysis, interview, draft, and verification.",
		"Mention style analysis, brief interviews, and final draft checks.",
		"Use a failed timeout and a fixed follow-up question as concrete examples.",
		"Compare elapsed seconds, style score, verification result, and generated length.",
		"Use the author's personal history as a musician and engineer.",
		"Avoid cloud-only assumptions.",
		"3000字前後 with six sections.",
		"Practical and introspective.",
		"local LLM, article draft, verification.",
		"Start with friction, move to workflow design, and close with one small next step.",
	}
}

func sessionHasQuestion(questions []domain.ArticleQuestion, id string) bool {
	for _, question := range questions {
		if question.ID == id {
			return true
		}
	}
	return false
}

type staticFollowUpGenerator struct {
	text string
	err  error
}

func (g staticFollowUpGenerator) GenerateFollowUp(ctx context.Context, session domain.ArticleBriefSession, target domain.ArticleQuestion, answer domain.BriefAnswer, followUpIndex int) (string, error) {
	return g.text, g.err
}
