package brief

import (
	"fmt"
	"strings"
)

var deepDivePriority = []string{
	QuestionIDOpeningEpisode,
	QuestionIDMustInclude,
	QuestionIDExpectedReaderAction,
	QuestionIDToneStance,
}

// CurrentQuestion returns the next unanswered fixed or deep-dive question.
func (s ArticleBriefSession) CurrentQuestion() (ArticleQuestion, bool) {
	if s.Completed || s.Phase == InterviewPhaseCompleted {
		return ArticleQuestion{}, false
	}
	for _, question := range s.Questions {
		if question.FlowType != QuestionFlowMain {
			continue
		}
		if _, ok := s.AnswerForQuestion(question.ID); !ok {
			return question, true
		}
	}
	if next, ok := s.NextDeepDiveQuestion(); ok {
		return next, true
	}
	return ArticleQuestion{}, false
}

// RecordAnswer appends a validated answer for the session's current question.
func (s *ArticleBriefSession) RecordAnswer(content string) (ArticleQuestion, error) {
	question, ok := s.CurrentQuestion()
	if !ok {
		return ArticleQuestion{}, fmt.Errorf("session has no active question")
	}
	answer, err := NewBriefAnswer(question, content)
	if err != nil {
		return ArticleQuestion{}, err
	}
	if question.FlowType == QuestionFlowMain {
		if _, ok := s.AnswerForQuestion(question.ID); ok {
			return ArticleQuestion{}, fmt.Errorf("question %q is already answered", question.ID)
		}
	}
	if question.FlowType == QuestionFlowDeepDiveFollowUp {
		if s.DeepDiveAnswerCount() >= MaxTotalFollowUps {
			return ArticleQuestion{}, fmt.Errorf("maximum total follow-up count reached")
		}
		if s.DeepDiveAnswerCountForTarget(question.TargetQuestionID) >= MaxFollowUpsPerTarget {
			return ArticleQuestion{}, fmt.Errorf("maximum follow-up count reached for target %q", question.TargetQuestionID)
		}
	}
	s.Answers = append(s.Answers, answer)
	s.refreshPhase()
	return question, nil
}

// NewBriefAnswer normalizes answer content and copies question metadata onto the answer.
func NewBriefAnswer(question ArticleQuestion, content string) (BriefAnswer, error) {
	content = strings.TrimSpace(content)
	if question.Required && content == "" {
		return BriefAnswer{}, fmt.Errorf("answer for question %q is required", question.ID)
	}
	return BriefAnswer{
		QuestionID:       question.ID,
		Content:          content,
		FlowType:         question.FlowType,
		TargetQuestionID: question.TargetQuestionID,
		FollowUpIndex:    question.FollowUpIndex,
	}, nil
}

// AnswerForQuestion returns the main answer for a fixed question.
func (s ArticleBriefSession) AnswerForQuestion(questionID string) (BriefAnswer, bool) {
	for _, answer := range s.Answers {
		if answer.FlowType == QuestionFlowMain && answer.QuestionID == questionID {
			return answer, true
		}
	}
	return BriefAnswer{}, false
}

// DeepDiveAnswers returns answers collected from generated follow-up questions.
func (s ArticleBriefSession) DeepDiveAnswers() []BriefAnswer {
	answers := make([]BriefAnswer, 0)
	for _, answer := range s.Answers {
		if answer.FlowType == QuestionFlowDeepDiveFollowUp {
			answers = append(answers, answer)
		}
	}
	return answers
}

// DeepDiveAnswerCount returns the total number of saved follow-up answers.
func (s ArticleBriefSession) DeepDiveAnswerCount() int {
	return len(s.DeepDiveAnswers())
}

// DeepDiveAnswerCountForTarget returns saved follow-up answers tied to one fixed question.
func (s ArticleBriefSession) DeepDiveAnswerCountForTarget(targetQuestionID string) int {
	count := 0
	for _, answer := range s.Answers {
		if answer.FlowType == QuestionFlowDeepDiveFollowUp && answer.TargetQuestionID == targetQuestionID {
			count++
		}
	}
	return count
}

// SelectDeepDiveTargets deterministically chooses high-value fixed answers worth deep-diving.
func (s ArticleBriefSession) SelectDeepDiveTargets() []ArticleQuestion {
	targets := make([]ArticleQuestion, 0, len(deepDivePriority))
	questions := questionByID(s.Questions)
	for _, questionID := range deepDivePriority {
		question, ok := questions[questionID]
		if !ok {
			continue
		}
		answer, ok := s.AnswerForQuestion(questionID)
		if !ok || strings.TrimSpace(answer.Content) == "" {
			continue
		}
		if !isDeepDiveCandidate(answer.Content) {
			continue
		}
		targets = append(targets, question)
	}
	return targets
}

// NextDeepDiveQuestion builds the next deterministic deep-dive follow-up question.
func (s ArticleBriefSession) NextDeepDiveQuestion() (ArticleQuestion, bool) {
	if s.DeepDiveSkipped || s.DeepDiveAnswerCount() >= MaxTotalFollowUps {
		return ArticleQuestion{}, false
	}
	for _, target := range s.SelectDeepDiveTargets() {
		count := s.DeepDiveAnswerCountForTarget(target.ID)
		if count >= MaxFollowUpsPerTarget {
			continue
		}
		targetAnswer, ok := s.AnswerForQuestion(target.ID)
		if !ok {
			continue
		}
		followUpIndex := count + 1
		return NewDeepDiveQuestion(target, followUpIndex, FallbackFollowUpText(target, targetAnswer, followUpIndex)), true
	}
	return ArticleQuestion{}, false
}

// NewDeepDiveQuestion creates a generated follow-up question anchored to a fixed question.
func NewDeepDiveQuestion(target ArticleQuestion, followUpIndex int, text string) ArticleQuestion {
	text = strings.TrimSpace(text)
	if !IsAllowedFollowUpQuestion(text) {
		text = "What concrete detail should the article add to make this answer useful?"
	}
	return ArticleQuestion{
		ID:               fmt.Sprintf("%s_follow_up_%d", target.ID, followUpIndex),
		Text:             text,
		FlowType:         QuestionFlowDeepDiveFollowUp,
		Required:         true,
		TargetField:      target.TargetField,
		TargetQuestionID: target.ID,
		FollowUpIndex:    followUpIndex,
	}
}

// FallbackFollowUpText returns a safe rule-based question when generated wording is unavailable.
func FallbackFollowUpText(target ArticleQuestion, answer BriefAnswer, followUpIndex int) string {
	switch target.ID {
	case QuestionIDOpeningEpisode:
		if followUpIndex == 1 {
			return "What specific scene from that opening episode should the reader see first?"
		}
		return "What emotion at the time should the article make clear?"
	case QuestionIDMustInclude:
		if followUpIndex == 1 {
			return "Which included point needs the most concrete detail, and what detail should be used?"
		}
		return "What lesson should the reader take from that required point?"
	case QuestionIDExpectedReaderAction:
		if followUpIndex == 1 {
			return "What reason should make the reader want to take that action?"
		}
		return "What concrete first step should the reader imagine after reading?"
	case QuestionIDToneStance:
		if followUpIndex == 1 {
			return "What stance should the article explain most carefully?"
		}
		return "What experience should support that tone or stance?"
	default:
		return "What concrete detail should the article add to make this answer useful?"
	}
}

// IsAllowedFollowUpQuestion checks that a generated follow-up is open-ended enough for the workflow.
func IsAllowedFollowUpQuestion(text string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	if trimmed == "" {
		return false
	}
	yesNoPrefixes := []string{
		"do ", "does ", "did ", "is ", "are ", "was ", "were ", "can ", "could ", "should ", "would ", "will ", "have ", "has ",
		"which is ", "which are ",
	}
	for _, prefix := range yesNoPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return false
		}
	}
	if strings.Contains(trimmed, " or ") {
		return false
	}
	return true
}

// MarkDeepDiveSkipped allows completion when the user explicitly skips deep dives.
func (s *ArticleBriefSession) MarkDeepDiveSkipped() {
	s.DeepDiveSkipped = true
	s.refreshPhase()
}

// CanComplete reports whether all minimum article-brief conditions are satisfied.
func (s ArticleBriefSession) CanComplete() bool {
	if strings.TrimSpace(s.StyleProfileID) == "" {
		return false
	}
	required := []string{
		QuestionIDTheme,
		QuestionIDReader,
		QuestionIDExpectedReaderAction,
		QuestionIDMustInclude,
	}
	for _, questionID := range required {
		answer, ok := s.AnswerForQuestion(questionID)
		if !ok || strings.TrimSpace(answer.Content) == "" {
			return false
		}
	}
	if s.hasAnsweredAllFixedQuestions() && !s.DeepDiveSkipped && len(s.SelectDeepDiveTargets()) > 0 && s.DeepDiveAnswerCount() == 0 {
		return false
	}
	return true
}

// Complete marks the session completed and returns the assembled brief.
func (s *ArticleBriefSession) Complete() (ArticleBrief, error) {
	if !s.CanComplete() {
		return ArticleBrief{}, fmt.Errorf("brief session is not complete")
	}
	brief := s.AssembleBrief()
	s.Phase = InterviewPhaseCompleted
	s.Completed = true
	return brief, nil
}

// AssembleBrief builds an ArticleBrief from the fixed and deep-dive answers.
func (s ArticleBriefSession) AssembleBrief() ArticleBrief {
	get := func(questionID string) string {
		answer, ok := s.AnswerForQuestion(questionID)
		if !ok {
			return ""
		}
		return answer.Content
	}
	targetLengthStructure := get(QuestionIDTargetLengthStructure)
	if strings.TrimSpace(targetLengthStructure) == "" {
		targetLengthStructure = DefaultTargetLengthStructure
	}
	return ArticleBrief{
		StyleProfileID:        s.StyleProfileID,
		Theme:                 get(QuestionIDTheme),
		OpeningEpisode:        get(QuestionIDOpeningEpisode),
		Reader:                get(QuestionIDReader),
		ExpectedReaderAction:  get(QuestionIDExpectedReaderAction),
		MustInclude:           get(QuestionIDMustInclude),
		Exclusions:            get(QuestionIDExclusions),
		TargetLengthStructure: targetLengthStructure,
		ToneStance:            get(QuestionIDToneStance),
		DeepDives:             s.DeepDiveAnswers(),
	}
}

func (s ArticleBriefSession) hasAnsweredAllFixedQuestions() bool {
	for _, question := range s.Questions {
		if question.FlowType != QuestionFlowMain {
			continue
		}
		if _, ok := s.AnswerForQuestion(question.ID); !ok {
			return false
		}
	}
	return true
}

func (s *ArticleBriefSession) refreshPhase() {
	if s.Completed {
		s.Phase = InterviewPhaseCompleted
		return
	}
	if !s.hasAnsweredAllFixedQuestions() {
		s.Phase = InterviewPhaseFixedQuestions
		return
	}
	if s.CanComplete() && (s.DeepDiveSkipped || s.DeepDiveAnswerCount() >= MaxTotalFollowUps || !s.hasAvailableDeepDive()) {
		s.Phase = InterviewPhaseCompleted
		s.Completed = true
		return
	}
	s.Phase = InterviewPhaseDeepDive
}

func (s ArticleBriefSession) hasAvailableDeepDive() bool {
	if s.DeepDiveSkipped || s.DeepDiveAnswerCount() >= MaxTotalFollowUps {
		return false
	}
	for _, target := range s.SelectDeepDiveTargets() {
		if s.DeepDiveAnswerCountForTarget(target.ID) < MaxFollowUpsPerTarget {
			return true
		}
	}
	return false
}

func questionByID(questions []ArticleQuestion) map[string]ArticleQuestion {
	result := make(map[string]ArticleQuestion, len(questions))
	for _, question := range questions {
		result[question.ID] = question
	}
	return result
}

func isDeepDiveCandidate(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	runes := len([]rune(content))
	if runes > 180 {
		return false
	}
	vagueAnswers := []string{"none", "nothing", "n/a", "unsure", "not sure", "まだない", "なし", "特になし"}
	lower := strings.ToLower(content)
	for _, vague := range vagueAnswers {
		if lower == vague {
			return false
		}
	}
	return true
}
