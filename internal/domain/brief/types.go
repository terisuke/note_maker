package brief

import (
	"fmt"
	"strings"

	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	"github.com/teradakousuke/note_maker/internal/domain/persona"
)

const (
	QuestionIDTheme                 = "theme"
	QuestionIDOpeningEpisode        = "opening_episode"
	QuestionIDReader                = "reader"
	QuestionIDExpectedReaderAction  = "expected_reader_action"
	QuestionIDMustInclude           = "must_include"
	QuestionIDPersonalContext       = "personal_context"
	QuestionIDExclusions            = "exclusions"
	QuestionIDTargetLengthStructure = "target_length_structure"
	QuestionIDToneStance            = "tone_stance"
	QuestionIDStoryArc              = "story_arc"
	QuestionIDTargetStack           = "target_stack"
	QuestionIDTechnicalProof        = "technical_proof"
	QuestionIDHomepageCTA           = "homepage_cta"
	QuestionIDHomepageTrust         = "homepage_trust"
	QuestionIDCloudiaViewpoint      = "cloudia_viewpoint"

	MaxFollowUpsPerTarget = 2
	MaxTotalFollowUps     = 4

	DefaultTargetLengthStructure = "3000字前後。導入、違和感、背景、実装と検証、読者への提案、結論で構成する"
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
	PersonaID             string
	OutputFormatID        string
	Theme                 string
	OpeningEpisode        string
	Reader                string
	ExpectedReaderAction  string
	MustInclude           string
	PersonalContext       string
	Exclusions            string
	TargetLengthStructure string
	ToneStance            string
	DeepDives             []BriefAnswer
	CustomAnswers         []BriefAnswer
}

// ArticleBriefSession owns article-interview state.
type ArticleBriefSession struct {
	ID              string
	StyleProfileID  string
	PersonaID       string
	OutputFormatID  string
	ParentSessionID string
	Phase           InterviewPhase
	Questions       []ArticleQuestion
	Answers         []BriefAnswer
	Completed       bool
	DeepDiveSkipped bool
}

// NewArticleBriefSession creates a session with the deterministic fixed question set.
func NewArticleBriefSession(id, styleProfileID string) (ArticleBriefSession, error) {
	return NewArticleBriefSessionWithQuestions(id, styleProfileID, FixedQuestions())
}

// NewArticleBriefSessionWithQuestions creates a session with a caller-provided question set.
func NewArticleBriefSessionWithQuestions(id, styleProfileID string, questions []ArticleQuestion) (ArticleBriefSession, error) {
	return NewArticleBriefSessionWithOptions(id, styleProfileID, persona.IDTerisuke, outputformat.IDNoteArticle, "", questions)
}

// NewArticleBriefSessionWithOptions creates a session with persona/format metadata.
func NewArticleBriefSessionWithOptions(id, styleProfileID, personaID, outputFormatID, parentSessionID string, questions []ArticleQuestion) (ArticleBriefSession, error) {
	if strings.TrimSpace(id) == "" {
		return ArticleBriefSession{}, fmt.Errorf("session id is required")
	}
	if strings.TrimSpace(styleProfileID) == "" {
		return ArticleBriefSession{}, fmt.Errorf("style profile id is required")
	}
	questions = NormalizeQuestions(questions)
	if len(questions) == 0 {
		return ArticleBriefSession{}, fmt.Errorf("questions are required")
	}
	return ArticleBriefSession{
		ID:              strings.TrimSpace(id),
		StyleProfileID:  strings.TrimSpace(styleProfileID),
		PersonaID:       persona.NormalizeID(personaID),
		OutputFormatID:  outputformat.NormalizeID(outputFormatID),
		ParentSessionID: strings.TrimSpace(parentSessionID),
		Phase:           InterviewPhaseFixedQuestions,
		Questions:       questions,
	}, nil
}

// NormalizeQuestions keeps only valid main questions and fills missing metadata.
func NormalizeQuestions(questions []ArticleQuestion) []ArticleQuestion {
	normalized := make([]ArticleQuestion, 0, len(questions))
	seen := map[string]bool{}
	for _, question := range questions {
		question.ID = strings.TrimSpace(question.ID)
		question.Text = strings.TrimSpace(question.Text)
		if question.ID == "" || question.Text == "" || seen[question.ID] {
			continue
		}
		if question.FlowType == "" {
			question.FlowType = QuestionFlowMain
		}
		if question.FlowType != QuestionFlowMain {
			continue
		}
		if question.TargetField == "" {
			question.TargetField = "custom"
		}
		seen[question.ID] = true
		normalized = append(normalized, question)
	}
	return normalized
}

// FixedQuestions returns the deterministic main interview questions.
func FixedQuestions() []ArticleQuestion {
	return []ArticleQuestion{
		{
			ID:          QuestionIDTheme,
			Text:        "記事の中心テーマは何ですか？",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "theme",
		},
		{
			ID:          QuestionIDOpeningEpisode,
			Text:        "記事の導入に置く具体的な体験や場面は何ですか？",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "opening_episode",
		},
		{
			ID:          QuestionIDReader,
			Text:        "この記事を届けたい読者は誰ですか？",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "reader",
		},
		{
			ID:          QuestionIDExpectedReaderAction,
			Text:        "読後に読者へどんな変化や行動を起こしてほしいですか？",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "expected_reader_action",
		},
		{
			ID:          QuestionIDMustInclude,
			Text:        "記事に必ず含める論点、事実、手順は何ですか？",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "must_include",
		},
		{
			ID:          QuestionIDPersonalContext,
			Text:        "著者本人の経験、肩書き、失敗、価値観など、記事に入れるべき属人的な文脈は何ですか？",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "personal_context",
		},
		{
			ID:          QuestionIDExclusions,
			Text:        "記事に含めないこと、避けたい表現、断言しないことは何ですか？",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "exclusions",
		},
		{
			ID:          QuestionIDTargetLengthStructure,
			Text:        "目標文字数と記事構成を指定してください。例: 3000字前後、導入・背景・実装・検証・提案・結論。",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "target_length_structure",
		},
		{
			ID:          QuestionIDToneStance,
			Text:        "記事のトーンや立場はどうしますか？内省、技術解説、実用、物語性の比重も指定してください。",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "tone_stance",
		},
	}
}

// ComposeFixedQuestions returns the server-side interview template for a persona and output format.
func ComposeFixedQuestions(personaID, outputFormatID string) []ArticleQuestion {
	personaID = persona.NormalizeID(personaID)
	if strings.TrimSpace(outputFormatID) == "" {
		if item, ok := persona.DefaultRegistry().Get(personaID); ok {
			outputFormatID = item.DefaultFormat
		}
	}
	outputFormatID = outputformat.NormalizeID(outputFormatID)
	if personaID == persona.IDTerisuke && outputFormatID == outputformat.IDNoteArticle {
		return FixedQuestions()
	}

	questions := FixedQuestions()
	questions = append(questions, formatExtensionQuestions(outputFormatID)...)
	questions = append(questions, personaExtensionQuestions(personaID)...)
	return NormalizeQuestions(questions)
}

func formatExtensionQuestions(outputFormatID string) []ArticleQuestion {
	switch outputFormatID {
	case outputformat.IDNoteArticle:
		return narrativeExtensionQuestions()
	case outputformat.IDMarkdownBlog, outputformat.IDZennArticle, outputformat.IDQiitaArticle:
		return technicalExtensionQuestions()
	case outputformat.IDHomepageSection:
		return homepageExtensionQuestions()
	default:
		return nil
	}
}

func narrativeExtensionQuestions() []ArticleQuestion {
	return []ArticleQuestion{
		{
			ID:          QuestionIDStoryArc,
			Text:        "読み物として印象に残すため、どんな感情の流れやオチを置きますか？",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
		},
	}
}

func technicalExtensionQuestions() []ArticleQuestion {
	return []ArticleQuestion{
		{
			ID:          QuestionIDTargetStack,
			Text:        "対象にする技術スタック、言語、ライブラリ、実行環境、前提バージョンは何ですか？",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "custom",
		},
		{
			ID:          QuestionIDTechnicalProof,
			Text:        "記事内で示す再現手順、コード例、検証結果、失敗例は何ですか？",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
		},
	}
}

func homepageExtensionQuestions() []ArticleQuestion {
	return []ArticleQuestion{
		{
			ID:          QuestionIDHomepageCTA,
			Text:        "このWebセクションを読んだ人に押してほしいCTAや次の行動は何ですか？",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "custom",
		},
		{
			ID:          QuestionIDHomepageTrust,
			Text:        "サービスや会社への信頼につながる実績、根拠、約束として何を入れますか？",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
		},
	}
}

func personaExtensionQuestions(personaID string) []ArticleQuestion {
	switch personaID {
	case persona.IDCloudia:
		return []ArticleQuestion{
			{
				ID:          QuestionIDCloudiaViewpoint,
				Text:        "クラウディアならではの視点や感想として、どんな驚き、楽しさ、つまずきを入れますか？",
				FlowType:    QuestionFlowMain,
				Required:    false,
				TargetField: "custom",
			},
		}
	default:
		return nil
	}
}
