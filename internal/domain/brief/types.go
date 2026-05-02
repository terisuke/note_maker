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
	QuestionIDReaderProblem         = "reader_problem"
	QuestionIDExpectedReaderAction  = "expected_reader_action"
	QuestionIDKeyTakeaway           = "key_takeaway"
	QuestionIDMustInclude           = "must_include"
	QuestionIDConcreteExample       = "concrete_example"
	QuestionIDEvidence              = "evidence"
	QuestionIDPersonalContext       = "personal_context"
	QuestionIDExclusions            = "exclusions"
	QuestionIDTargetLengthStructure = "target_length_structure"
	QuestionIDToneStance            = "tone_stance"
	QuestionIDTitleKeywords         = "title_keywords"
	QuestionIDStoryArc              = "story_arc"
	QuestionIDTargetStack           = "target_stack"
	QuestionIDPrerequisiteKnowledge = "prerequisite_knowledge"
	QuestionIDTechnicalProof        = "technical_proof"
	QuestionIDCodeExamples          = "code_examples"
	QuestionIDReferences            = "references"
	QuestionIDCorBlogPurpose        = "cor_blog_purpose"
	QuestionIDCorBlogNextAction     = "cor_blog_next_action"
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
			Text:        "この記事で一番伝えたいことを、ひとことで書くと何ですか？",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "theme",
		},
		{
			ID:          QuestionIDOpeningEpisode,
			Text:        "冒頭で使えそうな出来事や場面はありますか？いつ・どこで・何が起きましたか？",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "opening_episode",
		},
		{
			ID:          QuestionIDReader,
			Text:        "誰に向けて書きますか？例: これから試す人、社内メンバー、同じ悩みのある人。",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "reader",
		},
		{
			ID:          QuestionIDReaderProblem,
			Text:        "その読者は今、何に困っている・迷っていると思いますか？",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
		},
		{
			ID:          QuestionIDExpectedReaderAction,
			Text:        "読み終わった後、その人にまず何をしてほしいですか？",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "expected_reader_action",
		},
		{
			ID:          QuestionIDKeyTakeaway,
			Text:        "読者に一番持ち帰ってほしい言葉や考えは何ですか？",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
		},
		{
			ID:          QuestionIDMustInclude,
			Text:        "絶対に入れたい事実・手順・名前・数字を箇条書きで教えてください。",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "must_include",
		},
		{
			ID:          QuestionIDConcreteExample,
			Text:        "その話を伝えるための具体例、失敗例、画面、コード、会話などはありますか？",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
		},
		{
			ID:          QuestionIDEvidence,
			Text:        "根拠として出せる結果・数字・比較・リンク・観察はありますか？なければ「なし」でOKです。",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
		},
		{
			ID:          QuestionIDPersonalContext,
			Text:        "あなた自身はなぜこの話を書きたいですか？経験・問題意識・違和感を短く教えてください。",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "personal_context",
		},
		{
			ID:          QuestionIDExclusions,
			Text:        "書かないこと、避けたい言い方、まだ断言しないことはありますか？なければ「なし」でOKです。",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "exclusions",
		},
		{
			ID:          QuestionIDTargetLengthStructure,
			Text:        "長さと構成の希望はありますか？例: 1500字で軽く、3000字で詳しく、導入→手順→結果。",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "target_length_structure",
		},
		{
			ID:          QuestionIDToneStance,
			Text:        "文章の雰囲気はどうしますか？例: やさしく、熱量高め、冷静な技術報告、社内向け。",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "tone_stance",
		},
		{
			ID:          QuestionIDTitleKeywords,
			Text:        "タイトルや見出しに入れたい言葉はありますか？なければ「未定」でOKです。",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
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

	questions := FixedQuestions()
	questions = append(questions, formatExtensionQuestions(outputFormatID)...)
	questions = append(questions, personaExtensionQuestions(personaID)...)
	return NormalizeQuestions(questions)
}

func formatExtensionQuestions(outputFormatID string) []ArticleQuestion {
	switch outputFormatID {
	case outputformat.IDNoteArticle:
		return narrativeExtensionQuestions()
	case outputformat.IDMarkdownBlog:
		return append(technicalExtensionQuestions(), companyBlogExtensionQuestions()...)
	case outputformat.IDZennArticle, outputformat.IDQiitaArticle:
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
			Text:        "noteらしい読み物にするなら、どんな順番で気持ちや発見を見せますか？例: 違和感→試したこと→気づき→読者への一言。",
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
			Text:        "扱う技術・ツール・言語・バージョンは何ですか？わかる範囲でOKです。",
			FlowType:    QuestionFlowMain,
			Required:    true,
			TargetField: "custom",
		},
		{
			ID:          QuestionIDPrerequisiteKnowledge,
			Text:        "読者はどこまで知っている前提にしますか？初心者向けか、経験者向けかも教えてください。",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
		},
		{
			ID:          QuestionIDTechnicalProof,
			Text:        "再現手順や検証結果として、どこまで記事に載せますか？",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
		},
		{
			ID:          QuestionIDCodeExamples,
			Text:        "コード例・コマンド・設定ファイルを載せますか？載せるならどの部分ですか？",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
		},
		{
			ID:          QuestionIDReferences,
			Text:        "参考リンク、公式ドキュメント、過去記事など、記事からリンクしたいものはありますか？",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
		},
	}
}

func companyBlogExtensionQuestions() []ArticleQuestion {
	return []ArticleQuestion{
		{
			ID:          QuestionIDCorBlogPurpose,
			Text:        "自社ブログとして、今回は技術知見の報告ですか？社員や採用候補へのビジョン共有ですか？",
			FlowType:    QuestionFlowMain,
			Required:    false,
			TargetField: "custom",
		},
		{
			ID:          QuestionIDCorBlogNextAction,
			Text:        "会社ブログとして、読者に最後に何を感じてほしい・相談してほしいですか？",
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
