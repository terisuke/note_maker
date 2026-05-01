package brief

import (
	"fmt"
	"strings"
)

const (
	QuestionIDTheme                 = "theme"
	QuestionIDOpeningEpisode        = "opening_episode"
	QuestionIDReader                = "reader"
	QuestionIDExpectedReaderAction  = "expected_reader_action"
	QuestionIDMustInclude           = "must_include"
	QuestionIDExclusions            = "exclusions"
	QuestionIDTargetLengthStructure = "target_length_structure"
	QuestionIDToneStance            = "tone_stance"

	MaxFollowUpsPerTarget = 2
	MaxTotalFollowUps     = 4

	DefaultTargetLengthStructure = "1500-2000 words with an introduction, body, and conclusion"
)

// QuestionFlowType identifies how an interview question participates in the brief flow.
type QuestionFlowType string

const (
	QuestionFlowMain               QuestionFlowType = "main"
	QuestionFlowDeepDivePermission QuestionFlowType = "deep_dive_permission"
	QuestionFlowDeepDiveFollowUp   QuestionFlowType = "deep_dive_follow_up"
	QuestionFlowCompleted          QuestionFlowType = "completed"
)

// InterviewPhase is the coarse state of an article brief interview.
type InterviewPhase string

const (
	InterviewPhaseFixedQuestions InterviewPhase = "fixed_questions"
	InterviewPhaseDeepDive       InterviewPhase = "deep_dive"
	InterviewPhaseCompleted      InterviewPhase = "completed"
)

// ArticleQuestion is a fixed or generated interview question.
type ArticleQuestion struct {
	ID               string
	Text             string
	FlowType         QuestionFlowType
	Required         bool
	TargetField      string
	TargetQuestionID string
	FollowUpIndex    int
}

// BriefAnswer stores one answer and the question/deep-dive metadata needed to audit it.
type BriefAnswer struct {
	QuestionID       string
	Content          string
	FlowType         QuestionFlowType
	TargetQuestionID string
	FollowUpIndex    int
}

// ArticleBrief is the structured requirement set assembled from a completed interview.
type ArticleBrief struct {
	StyleProfileID        string
	Theme                 string
	OpeningEpisode        string
	Reader                string
	ExpectedReaderAction  string
	MustInclude           string
	Exclusions            string
	TargetLengthStructure string
	ToneStance            string
	DeepDives             []BriefAnswer
}

// ArticleBriefSession owns article-interview state.
type ArticleBriefSession struct {
	ID              string
	StyleProfileID  string
	Phase           InterviewPhase
	Questions       []ArticleQuestion
	Answers         []BriefAnswer
	Completed       bool
	DeepDiveSkipped bool
}

// NewArticleBriefSession creates a session with the deterministic fixed question set.
func NewArticleBriefSession(id, styleProfileID string) (ArticleBriefSession, error) {
	if strings.TrimSpace(id) == "" {
		return ArticleBriefSession{}, fmt.Errorf("session id is required")
	}
	if strings.TrimSpace(styleProfileID) == "" {
		return ArticleBriefSession{}, fmt.Errorf("style profile id is required")
	}
	return ArticleBriefSession{
		ID:             strings.TrimSpace(id),
		StyleProfileID: strings.TrimSpace(styleProfileID),
		Phase:          InterviewPhaseFixedQuestions,
		Questions:      FixedQuestions(),
	}, nil
}

// FixedQuestions returns the deterministic main interview questions.
func FixedQuestions() []ArticleQuestion {
	return []ArticleQuestion{
		{
			ID:          QuestionIDTheme,
			Text:        "What is the article's core theme?",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "theme",
		},
		{
			ID:          QuestionIDOpeningEpisode,
			Text:        "What concrete experience or episode should open the article?",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "opening_episode",
		},
		{
			ID:          QuestionIDReader,
			Text:        "Who is the reader?",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "reader",
		},
		{
			ID:          QuestionIDExpectedReaderAction,
			Text:        "What should the reader feel or do after reading?",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "expected_reader_action",
		},
		{
			ID:          QuestionIDMustInclude,
			Text:        "What must be included?",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "must_include",
		},
		{
			ID:          QuestionIDExclusions,
			Text:        "What must be excluded?",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "exclusions",
		},
		{
			ID:          QuestionIDTargetLengthStructure,
			Text:        "What target length and structure should be used?",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "target_length_structure",
		},
		{
			ID:          QuestionIDToneStance,
			Text:        "Should the article be more introspective, technical, narrative, or practical?",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "tone_stance",
		},
	}
}
