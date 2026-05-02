package brief

import (
	"context"
	"errors"
	"testing"

	domain "github.com/teradakousuke/note_maker/internal/domain/brief"
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

func fixedAnswers() []string {
	return []string{
		"Local article generation with a small deterministic workflow.",
		"Open with a failed local LLM run that timed out while drafting.",
		"Solo developers who write note.com articles with local tools.",
		"They should try a three-phase workflow before drafting.",
		"Mention style analysis, brief interviews, and final draft checks.",
		"Use the author's personal history as a musician and engineer.",
		"Avoid cloud-only assumptions.",
		"3000字前後 with six sections.",
		"Practical and introspective.",
	}
}

type staticFollowUpGenerator struct {
	text string
	err  error
}

func (g staticFollowUpGenerator) GenerateFollowUp(ctx context.Context, session domain.ArticleBriefSession, target domain.ArticleQuestion, answer domain.BriefAnswer, followUpIndex int) (string, error) {
	return g.text, g.err
}
