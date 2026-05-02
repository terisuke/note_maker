package brief

import (
	"context"
	"fmt"
	"strings"

	domain "github.com/teradakousuke/note_maker/internal/domain/brief"
)

// FollowUpGenerator can phrase deep-dive questions; services fall back to domain rules on error.
type FollowUpGenerator interface {
	GenerateFollowUp(ctx context.Context, session domain.ArticleBriefSession, target domain.ArticleQuestion, answer domain.BriefAnswer, followUpIndex int) (string, error)
}

// InterviewService drives one-question-at-a-time article brief sessions.
type InterviewService struct {
	followUpGenerator FollowUpGenerator
}

// StartSessionInput contains required session creation data.
type StartSessionInput struct {
	SessionID      string
	StyleProfileID string
	Questions      []domain.ArticleQuestion
}

// InterviewResult contains either the next question or a completed brief.
type InterviewResult struct {
	Session      domain.ArticleBriefSession
	NextQuestion *domain.ArticleQuestion
	Brief        *domain.ArticleBrief
	Completed    bool
}

// NewInterviewService creates an article brief interview application service.
func NewInterviewService(followUpGenerator FollowUpGenerator) *InterviewService {
	return &InterviewService{followUpGenerator: followUpGenerator}
}

// StartSession creates a session and returns the first fixed question.
func (s *InterviewService) StartSession(input StartSessionInput) (InterviewResult, error) {
	if len(input.Questions) == 0 {
		input.Questions = domain.FixedQuestions()
	}
	session, err := domain.NewArticleBriefSessionWithQuestions(input.SessionID, input.StyleProfileID, input.Questions)
	if err != nil {
		return InterviewResult{}, err
	}
	return s.buildResult(context.Background(), session)
}

// Answer accepts one answer and returns either the next question or the completed brief.
func (s *InterviewService) Answer(ctx context.Context, session domain.ArticleBriefSession, content string) (InterviewResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if session.Completed || session.Phase == domain.InterviewPhaseCompleted {
		return InterviewResult{}, fmt.Errorf("brief session is already completed")
	}
	current, err := session.RecordAnswer(content)
	if err != nil {
		return InterviewResult{}, err
	}
	if current.FlowType == domain.QuestionFlowDeepDiveFollowUp {
		if !sessionHasAvailableRequiredFollowUp(session) {
			brief, err := session.Complete()
			if err != nil {
				return InterviewResult{}, err
			}
			return InterviewResult{
				Session:   session,
				Brief:     &brief,
				Completed: true,
			}, nil
		}
	}
	return s.buildResult(ctx, session)
}

func (s *InterviewService) buildResult(ctx context.Context, session domain.ArticleBriefSession) (InterviewResult, error) {
	if question, ok := session.CurrentQuestion(); ok {
		question = s.withGeneratedFollowUp(ctx, session, question)
		return InterviewResult{
			Session:      session,
			NextQuestion: &question,
		}, nil
	}
	brief, err := session.Complete()
	if err != nil {
		return InterviewResult{}, err
	}
	return InterviewResult{
		Session:   session,
		Brief:     &brief,
		Completed: true,
	}, nil
}

func (s *InterviewService) withGeneratedFollowUp(ctx context.Context, session domain.ArticleBriefSession, question domain.ArticleQuestion) domain.ArticleQuestion {
	if question.FlowType != domain.QuestionFlowDeepDiveFollowUp || s.followUpGenerator == nil {
		return question
	}
	target, answer, ok := followUpContext(session, question)
	if !ok {
		return question
	}
	generated, err := s.followUpGenerator.GenerateFollowUp(ctx, session, target, answer, question.FollowUpIndex)
	if err != nil {
		return question
	}
	generated = strings.TrimSpace(generated)
	if !domain.IsAllowedFollowUpQuestion(generated) {
		return question
	}
	question.Text = generated
	return question
}

func followUpContext(session domain.ArticleBriefSession, question domain.ArticleQuestion) (domain.ArticleQuestion, domain.BriefAnswer, bool) {
	var target domain.ArticleQuestion
	foundTarget := false
	for _, candidate := range session.Questions {
		if candidate.ID == question.TargetQuestionID {
			target = candidate
			foundTarget = true
			break
		}
	}
	answer, foundAnswer := session.AnswerForQuestion(question.TargetQuestionID)
	return target, answer, foundTarget && foundAnswer
}

func sessionHasAvailableRequiredFollowUp(session domain.ArticleBriefSession) bool {
	if session.DeepDiveAnswerCount() >= domain.MaxTotalFollowUps {
		return false
	}
	for _, target := range session.SelectDeepDiveTargets() {
		if session.DeepDiveAnswerCountForTarget(target.ID) < domain.MaxFollowUpsPerTarget {
			return true
		}
	}
	return false
}
