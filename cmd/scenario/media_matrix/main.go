package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

const defaultOutputDir = "tmp/media_matrix"

var expectedSourceSelectors = []string{
	"note:cor_instrument",
	"zenn:cloudia",
	"qiita:Cloudia_Cor_Inc",
	"rss:https://cor-jp.com/rss.xml",
	"github:Cor-Incorporated/corsweb2024/src/content/blog/ja",
}

type matrixCase struct {
	ID                    string   `json:"id"`
	PersonaID             string   `json:"persona_id"`
	OutputFormatID        string   `json:"output_format_id"`
	Medium                string   `json:"medium"`
	Style                 string   `json:"style"`
	Theme                 string   `json:"theme"`
	OpeningEpisode        string   `json:"opening_episode"`
	Reader                string   `json:"reader"`
	ExpectedReaderAction  string   `json:"expected_reader_action"`
	MustInclude           string   `json:"must_include"`
	PersonalContext       string   `json:"personal_context"`
	Exclusions            string   `json:"exclusions"`
	TargetLengthStructure string   `json:"target_length_structure"`
	ToneStance            string   `json:"tone_stance"`
	SourceSelectors       []string `json:"source_selectors"`
	PromptMustContain     []string `json:"prompt_must_contain"`
}

type matrixOutput struct {
	GeneratedBy               string                  `json:"generated_by"`
	OfflineOnly               bool                    `json:"offline_only"`
	ExpectedSourceSelectors   []sourceSelectorResult  `json:"expected_source_selectors"`
	FixedQuestionTemplate     []questionTemplateEntry `json:"fixed_question_template"`
	ComposedQuestionTemplates []questionTemplateSet   `json:"composed_question_templates"`
	Cases                     []caseResult            `json:"cases"`
	ComparisonMetricsTemplate []string                `json:"comparison_metrics_template"`
}

type sourceSelectorResult struct {
	Selector string `json:"selector"`
	Found    bool   `json:"found"`
	Persona  string `json:"persona,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

type questionTemplateEntry struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	Required    bool   `json:"required"`
	TargetField string `json:"target_field"`
}

type questionTemplateSet struct {
	PersonaID      string                  `json:"persona_id"`
	OutputFormatID string                  `json:"output_format_id"`
	Questions      []questionTemplateEntry `json:"questions"`
}

type caseResult struct {
	ID                    string              `json:"id"`
	PersonaID             string              `json:"persona_id"`
	PersonaDisplayName    string              `json:"persona_display_name"`
	OutputFormatID        string              `json:"output_format_id"`
	OutputFormatName      string              `json:"output_format_name"`
	Medium                string              `json:"medium"`
	Style                 string              `json:"style"`
	Theme                 string              `json:"theme"`
	TargetLengthStructure string              `json:"target_length_structure"`
	SourceSelectors       []string            `json:"source_selectors"`
	BriefPath             string              `json:"brief_path"`
	PromptPath            string              `json:"prompt_path"`
	PromptChecks          []promptCheckResult `json:"prompt_checks"`
	QuestionIDs           []string            `json:"question_ids"`
	PlannedLLMCommand     string              `json:"planned_llm_command"`
	ExpectedMetrics       []string            `json:"expected_metrics"`
}

type promptCheckResult struct {
	Contains string `json:"contains"`
	Passed   bool   `json:"passed"`
}

func main() {
	outputDir := envOrDefault("SCENARIO_OUTPUT_DIR", defaultOutputDir)
	briefDir := filepath.Join(outputDir, "briefs")
	promptDir := filepath.Join(outputDir, "prompts")
	for _, dir := range []string{outputDir, briefDir, promptDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fatalf("create output dir %s: %v", dir, err)
		}
	}

	personas := personadomain.DefaultRegistry()
	formats := outputformat.DefaultRegistry()
	sourceResults := verifyExpectedSources(personas.List())
	questionTemplate := fixedQuestionTemplate()
	composedTemplates := composedQuestionTemplates()
	guide := scenarioGuide()

	results := make([]caseResult, 0, len(plannedCases()))
	for _, item := range plannedCases() {
		persona, ok := personas.Get(item.PersonaID)
		if !ok {
			fatalf("%s references unknown persona %s", item.ID, item.PersonaID)
		}
		format, ok := formats.Get(item.OutputFormatID)
		if !ok {
			fatalf("%s references unknown output format %s", item.ID, item.OutputFormatID)
		}
		verifyCaseSources(item, sourceResults)
		questions := briefdomain.ComposeFixedQuestions(item.PersonaID, item.OutputFormatID)
		questionIDs := questionIDsFromQuestions(questions)

		brief := buildBriefFromSession(item)
		if brief.PersonaID != persona.ID || brief.OutputFormatID != format.ID {
			fatalf("%s assembled mismatched brief persona/format", item.ID)
		}
		prompt := draftapp.BuildPromptForMode(guide, brief, persona, format)
		checks := verifyPrompt(item, persona, format, prompt)

		briefPath := filepath.Join(briefDir, item.ID+".json")
		promptPath := filepath.Join(promptDir, item.ID+".prompt.md")
		writeJSON(briefPath, brief)
		writeFile(promptPath, prompt)

		results = append(results, caseResult{
			ID:                    item.ID,
			PersonaID:             persona.ID,
			PersonaDisplayName:    persona.DisplayName,
			OutputFormatID:        format.ID,
			OutputFormatName:      format.DisplayName,
			Medium:                item.Medium,
			Style:                 item.Style,
			Theme:                 item.Theme,
			TargetLengthStructure: item.TargetLengthStructure,
			SourceSelectors:       append([]string(nil), item.SourceSelectors...),
			BriefPath:             briefPath,
			PromptPath:            promptPath,
			PromptChecks:          checks,
			QuestionIDs:           append([]string(nil), questionIDs...),
			PlannedLLMCommand:     plannedLLMCommand(outputDir, item.ID, briefPath),
			ExpectedMetrics: []string{
				"elapsed_seconds",
				"score",
				"passed",
				"verification_performed",
				"verification_passed",
				"runes",
			},
		})
	}

	matrix := matrixOutput{
		GeneratedBy:               "cmd/scenario/media_matrix",
		OfflineOnly:               true,
		ExpectedSourceSelectors:   sourceResults,
		FixedQuestionTemplate:     questionTemplate,
		ComposedQuestionTemplates: composedTemplates,
		Cases:                     results,
		ComparisonMetricsTemplate: []string{
			"case_id",
			"phase",
			"medium",
			"style",
			"target_length_structure",
			"elapsed_seconds",
			"score",
			"verification_passed",
			"output_path",
		},
	}
	writeJSON(filepath.Join(outputDir, "matrix.json"), matrix)
	writeFile(filepath.Join(outputDir, "cases.md"), casesMarkdown(results))

	fmt.Printf("media matrix scenario completed\n")
	fmt.Printf("offline_only=%v\n", matrix.OfflineOnly)
	fmt.Printf("source_selectors=%d\n", len(matrix.ExpectedSourceSelectors))
	fmt.Printf("question_template_ids=%d\n", len(matrix.FixedQuestionTemplate))
	fmt.Printf("composed_templates=%d\n", len(matrix.ComposedQuestionTemplates))
	fmt.Printf("cases=%d\n", len(matrix.Cases))
	fmt.Printf("matrix=%s\n", filepath.Join(outputDir, "matrix.json"))
	fmt.Printf("cases_markdown=%s\n", filepath.Join(outputDir, "cases.md"))
}

func verifyExpectedSources(personas []personadomain.Persona) []sourceSelectorResult {
	available := map[string]sourceSelectorResult{}
	for _, persona := range personas {
		for _, source := range persona.Sources {
			selector := sourceSelector(source)
			if selector == "" {
				continue
			}
			available[selector] = sourceSelectorResult{
				Selector: selector,
				Found:    true,
				Persona:  persona.ID,
				Kind:     source.Kind,
			}
		}
	}

	results := make([]sourceSelectorResult, 0, len(expectedSourceSelectors))
	for _, selector := range expectedSourceSelectors {
		result, ok := available[selector]
		if !ok {
			fatalf("expected source selector %s was not found in seeded personas", selector)
		}
		results = append(results, result)
	}
	return results
}

func sourceSelector(source personadomain.AuthorSource) string {
	kind := strings.TrimSpace(source.Kind)
	ref := strings.TrimSpace(source.Ref)
	url := strings.TrimSpace(source.URL)
	switch kind {
	case "note", "zenn", "qiita":
		if ref == "" {
			return ""
		}
		return kind + ":" + ref
	case "rss":
		if url == "" {
			return ""
		}
		return "rss:" + strings.TrimSuffix(url, "/")
	case "github":
		return githubSelector(url)
	default:
		return ""
	}
}

func githubSelector(url string) string {
	const marker = "github.com/"
	index := strings.Index(url, marker)
	if index < 0 {
		return ""
	}
	path := strings.TrimPrefix(url[index+len(marker):], "/")
	path = strings.TrimSuffix(path, "/")
	path = strings.Replace(path, "/tree/main/", "/", 1)
	path = strings.Replace(path, "/tree/master/", "/", 1)
	if path == "" {
		return ""
	}
	return "github:" + path
}

func fixedQuestionTemplate() []questionTemplateEntry {
	questions := briefdomain.FixedQuestions()
	entries := make([]questionTemplateEntry, 0, len(questions))
	requiredIDs := map[string]bool{
		briefdomain.QuestionIDTheme:                 true,
		briefdomain.QuestionIDReader:                true,
		briefdomain.QuestionIDExpectedReaderAction:  true,
		briefdomain.QuestionIDMustInclude:           true,
		briefdomain.QuestionIDPersonalContext:       true,
		briefdomain.QuestionIDTargetLengthStructure: true,
		briefdomain.QuestionIDToneStance:            true,
	}
	seen := map[string]bool{}
	for _, question := range questions {
		seen[question.ID] = true
		entries = append(entries, questionTemplateEntry{
			ID:          question.ID,
			Text:        question.Text,
			Required:    question.Required,
			TargetField: question.TargetField,
		})
	}
	for id := range requiredIDs {
		if !seen[id] {
			fatalf("fixed question template missing %s", id)
		}
	}
	return entries
}

func questionIDsFromTemplate(template []questionTemplateEntry) []string {
	ids := make([]string, 0, len(template))
	for _, question := range template {
		ids = append(ids, question.ID)
	}
	return ids
}

func questionIDsFromQuestions(questions []briefdomain.ArticleQuestion) []string {
	ids := make([]string, 0, len(questions))
	for _, question := range questions {
		ids = append(ids, question.ID)
	}
	return ids
}

func composedQuestionTemplates() []questionTemplateSet {
	personas := []string{personadomain.IDTerisuke, personadomain.IDCloudia}
	formats := []string{
		outputformat.IDNoteArticle,
		outputformat.IDMarkdownBlog,
		outputformat.IDZennArticle,
		outputformat.IDQiitaArticle,
		outputformat.IDHomepageSection,
	}
	result := make([]questionTemplateSet, 0, len(personas)*len(formats))
	for _, personaID := range personas {
		for _, formatID := range formats {
			questions := briefdomain.ComposeFixedQuestions(personaID, formatID)
			if personaID == personadomain.IDCloudia && formatID == outputformat.IDZennArticle {
				requireQuestion(questions, briefdomain.QuestionIDTargetStack)
				requireQuestion(questions, briefdomain.QuestionIDCloudiaViewpoint)
			}
			result = append(result, questionTemplateSet{
				PersonaID:      personaID,
				OutputFormatID: formatID,
				Questions:      questionTemplateEntries(questions),
			})
		}
	}
	return result
}

func questionTemplateEntries(questions []briefdomain.ArticleQuestion) []questionTemplateEntry {
	entries := make([]questionTemplateEntry, 0, len(questions))
	for _, question := range questions {
		entries = append(entries, questionTemplateEntry{
			ID:          question.ID,
			Text:        question.Text,
			Required:    question.Required,
			TargetField: question.TargetField,
		})
	}
	return entries
}

func requireQuestion(questions []briefdomain.ArticleQuestion, id string) {
	for _, question := range questions {
		if question.ID == id {
			return
		}
	}
	fatalf("composed question template missing %s", id)
}

func verifyCaseSources(item matrixCase, sources []sourceSelectorResult) {
	available := map[string]bool{}
	for _, source := range sources {
		available[source.Selector] = source.Found
	}
	for _, selector := range item.SourceSelectors {
		if !available[selector] {
			fatalf("%s references unavailable source selector %s", item.ID, selector)
		}
	}
}

func buildBriefFromSession(item matrixCase) briefdomain.ArticleBrief {
	session, err := briefdomain.NewArticleBriefSessionWithOptions(item.ID, "profile_media_matrix", item.PersonaID, item.OutputFormatID, "", briefdomain.ComposeFixedQuestions(item.PersonaID, item.OutputFormatID))
	if err != nil {
		fatalf("%s create brief session: %v", item.ID, err)
	}
	for _, question := range session.Questions {
		if _, err := session.RecordAnswer(answerForQuestion(item, question.ID)); err != nil {
			fatalf("%s answer %s: %v", item.ID, question.ID, err)
		}
	}
	session.MarkDeepDiveSkipped()
	brief, err := session.Complete()
	if err != nil {
		fatalf("%s complete brief session: %v", item.ID, err)
	}
	return brief
}

func answerForQuestion(item matrixCase, questionID string) string {
	switch questionID {
	case briefdomain.QuestionIDTheme:
		return item.Theme
	case briefdomain.QuestionIDOpeningEpisode:
		return item.OpeningEpisode
	case briefdomain.QuestionIDReader:
		return item.Reader
	case briefdomain.QuestionIDReaderProblem:
		return "媒体ごとの作法が違い、どの粒度で書けば読者に届くか迷っている。"
	case briefdomain.QuestionIDExpectedReaderAction:
		return item.ExpectedReaderAction
	case briefdomain.QuestionIDKeyTakeaway:
		return "媒体に合わせて、同じ知見でも入口と根拠の出し方を変える。"
	case briefdomain.QuestionIDMustInclude:
		return item.MustInclude
	case briefdomain.QuestionIDConcreteExample:
		return "実際の取得元、Markdown形式、検証コマンド、生成後の評価結果を例として出す。"
	case briefdomain.QuestionIDEvidence:
		return "シナリオCLIの出力、style score、verification PASS/NEEDS_REVIEW、生成文字数を根拠にする。"
	case briefdomain.QuestionIDPersonalContext:
		return item.PersonalContext
	case briefdomain.QuestionIDExclusions:
		return item.Exclusions
	case briefdomain.QuestionIDTargetLengthStructure:
		return item.TargetLengthStructure
	case briefdomain.QuestionIDToneStance:
		return item.ToneStance
	case briefdomain.QuestionIDTitleKeywords:
		return "AI駆動開発、媒体別、下書き、検証、Evo X2。"
	case briefdomain.QuestionIDStoryArc:
		return "導入の違和感から、実践で見えた発見へ進み、読者が次に試す一歩で締める。"
	case briefdomain.QuestionIDTargetStack:
		return "Go 1.26、OpenAI互換API、Ollama/Evo X2、Markdown validator、ローカルシナリオCLI。"
	case briefdomain.QuestionIDPrerequisiteKnowledge:
		return "GoとMarkdownの基礎は知っているが、媒体別の記法差分やローカルLLM運用はこれから試す読者。"
	case briefdomain.QuestionIDTechnicalProof:
		return "実行コマンド、JSON出力、本文長、style score、verification結果を比較表に残す。"
	case briefdomain.QuestionIDCodeExamples:
		return "必要ならMakefileターゲット、curl、JSONの抜粋を短く載せる。"
	case briefdomain.QuestionIDReferences:
		return "Zenn/Qiita公式Markdownガイド、corsweb2024のMarkdown記事、過去の検証ログ。"
	case briefdomain.QuestionIDCorBlogPurpose:
		return "技術知見の報告を主にし、社員や採用候補へ開発方針も伝える。"
	case briefdomain.QuestionIDCorBlogNextAction:
		return "Cor.incの開発文化と検証姿勢を理解し、相談や協業につなげてもらう。"
	case briefdomain.QuestionIDHomepageCTA:
		return "問い合わせまたは技術相談への導線を置き、検証可能な発信基盤を短く伝える。"
	case briefdomain.QuestionIDHomepageTrust:
		return "実装済みの媒体別fetcher、format validator、シナリオ評価を根拠として示す。"
	case briefdomain.QuestionIDCloudiaViewpoint:
		return "クラウディア視点では、つまずきポイントを明るく拾い、初心者が一緒に試せる楽しさを入れる。"
	default:
		return ""
	}
}

func verifyPrompt(item matrixCase, persona personadomain.Persona, format outputformat.OutputFormat, prompt string) []promptCheckResult {
	mustContain := []string{
		persona.DisplayName,
		format.DisplayName,
		item.Theme,
		item.TargetLengthStructure,
		"## 媒体別Markdownガイド",
	}
	mustContain = append(mustContain, item.PromptMustContain...)
	results := make([]promptCheckResult, 0, len(mustContain))
	for _, expected := range mustContain {
		passed := strings.Contains(prompt, expected)
		if !passed {
			fatalf("%s prompt missing %q", item.ID, expected)
		}
		results = append(results, promptCheckResult{Contains: expected, Passed: passed})
	}
	return results
}

func plannedLLMCommand(outputDir, caseID, briefPath string) string {
	return fmt.Sprintf("RUN_LOCAL_LLM_SCENARIO=1 ARTICLE_BRIEF_PATH=%s SCENARIO_OUTPUT_DIR=%s go run ./cmd/scenario/draft_generation", briefPath, filepath.Join(outputDir, "live", caseID))
}

func scenarioGuide() authordomain.WritingStyleGuide {
	return authordomain.WritingStyleGuide{
		ID:                   "guide_media_matrix",
		ProfileID:            "profile_media_matrix",
		Markdown:             "実体験、検証、読者への次の行動を媒体ごとに配分する。媒体が技術寄りなら手順と根拠を厚くし、noteやホームページでは読者の判断につながる文脈を厚くする。",
		PreferredFirstPerson: "僕",
		RecurringThemes:      []string{"AI", "検証", "発信", "実装"},
		ParagraphRhythm:      "媒体ごとの読み方に合わせて段落密度を変える。",
		SentenceRhythm:       "結論、根拠、次の行動が追える明快な文にする。",
		HeadingGuidance:      "読者が期待する粒度で具体的な見出しにする。",
		QuoteGuidance:        "引用は主張の転換点に限定する。",
		OpeningPatterns:      []string{"具体的な違和感から始める", "検証結果から始める", "読者の作業場面から始める"},
		ConclusionPatterns:   []string{"次の行動で締める", "検証の残課題で締める"},
	}
}

func plannedCases() []matrixCase {
	return []matrixCase{
		{
			ID:                    "terisuke_note_essay",
			PersonaID:             personadomain.IDTerisuke,
			OutputFormatID:        outputformat.IDNoteArticle,
			Medium:                "note",
			Style:                 "reflective essay",
			Theme:                 "AI時代に手触りある創作を残す理由",
			OpeningEpisode:        "ローカルLLMで下書きを速く作れた一方で、自分の違和感が抜けた原稿を読み返した場面",
			Reader:                "AIで発信を増やしたいが、自分の言葉が薄まることを気にしている個人クリエイター",
			ExpectedReaderAction:  "生成速度だけでなく、違和感や体験を先にメモしてからAIへ渡す",
			MustInclude:           "創作の手触り、AIを使う前の素材メモ、下書き後の読み返し、読者への提案",
			PersonalContext:       "音楽家、エンジニア、起業家として、便利さと表現の固有性の間で判断してきた経験",
			Exclusions:            "AI万能論、根拠のない効率化断言、クラウディア口調",
			TargetLengthStructure: "3000字前後。導入、違和感、背景、実践、読者への提案、結論",
			ToneStance:            "内省を中心に、体験から技術との付き合い方へ接続する",
			SourceSelectors:       []string{"note:cor_instrument"},
			PromptMustContain:     []string{"note.com", "frontmatter、HTML"},
		},
		{
			ID:                    "cor_blog_technical_report",
			PersonaID:             personadomain.IDTerisuke,
			OutputFormatID:        outputformat.IDMarkdownBlog,
			Medium:                "cor-jp.com company blog",
			Style:                 "technical report",
			Theme:                 "ソース別スタイル分析を記事生成に接続する実装報告",
			OpeningEpisode:        "note、Zenn、Qiita、RSS、GitHub Markdownを同じ検証表で見たときに本文密度の差が出た場面",
			Reader:                "Cor.incの開発チームと、記事生成パイプラインの実装判断を知りたい技術読者",
			ExpectedReaderAction:  "媒体ごとの入力ソースと検証指標を分けて、次の実装タスクを切れるようにする",
			MustInclude:           "selector一覧、RSSとGitHub Markdownの役割差、format validator、runtime/score/verificationの比較観点",
			PersonalContext:       "自社発信を継続可能な仕組みにするため、実装と編集判断を同じ場で扱っている背景",
			Exclusions:            "未計測の性能比較、ライブLLM結果の捏造、読者が再現できない手順",
			TargetLengthStructure: "2200-2800字。背景、実装、検証結果、リスク、次の実装",
			ToneStance:            "会社ブログとして断定口調で、実装判断と検証可能性を明確にする",
			SourceSelectors: []string{
				"rss:https://cor-jp.com/rss.xml",
				"github:Cor-Incorporated/corsweb2024/src/content/blog/ja",
			},
			PromptMustContain: []string{"corsweb2024", "category は ai / engineering / founder / lab", "実装判断"},
		},
		{
			ID:                    "cor_blog_vision_sharing",
			PersonaID:             personadomain.IDTerisuke,
			OutputFormatID:        outputformat.IDMarkdownBlog,
			Medium:                "cor-jp.com company blog",
			Style:                 "vision sharing",
			Theme:                 "生成AI時代のCor.inc発信基盤をどう育てるか",
			OpeningEpisode:        "複数媒体の出力形式を揃えたことで、会社として何を蓄積すべきかが見えた場面",
			Reader:                "Cor.incのメンバーと、会社の発信基盤に関心がある採用候補者",
			ExpectedReaderAction:  "発信を個人技にせず、検証と再利用ができる社内資産として扱う",
			MustInclude:           "会社ブログの役割、技術知見とビジョン共有の両立、媒体別ガイド、今後の運用方針",
			PersonalContext:       "創業者として、プロダクト開発と発信を同じ学習ループに入れたい意図",
			Exclusions:            "抽象的なスローガンだけの記事、採用広報だけに寄った表現",
			TargetLengthStructure: "1600-2200字。課題、方針、運用、期待する行動",
			ToneStance:            "社員へのビジョン共有として、落ち着いた断定と具体的な運用案を混ぜる",
			SourceSelectors: []string{
				"rss:https://cor-jp.com/rss.xml",
				"github:Cor-Incorporated/corsweb2024/src/content/blog/ja",
			},
			PromptMustContain: []string{"社員へのビジョン共有", "lang は必ず \"ja\""},
		},
		{
			ID:                    "cloudia_zenn_tutorial",
			PersonaID:             personadomain.IDCloudia,
			OutputFormatID:        outputformat.IDZennArticle,
			Medium:                "Zenn",
			Style:                 "tutorial",
			Theme:                 "Goで媒体別プロンプトを検証する小さなCLIを作る",
			OpeningEpisode:        "記事の出し先を変えたらfrontmatterや独自記法が混ざってエラーになった場面",
			Reader:                "Goで記事生成ツールを作っていて、Zenn向け出力を安定させたい開発者",
			ExpectedReaderAction:  "Zenn用のfrontmatter、topics、messageブロックを分けて検証する",
			MustInclude:           "前提、実装手順、コード例、Zenn独自記法、つまずきポイント",
			PersonalContext:       "クラウディアとして、初心者にも楽しく手順を追える技術解説にする",
			Exclusions:            "Qiitaの:::note、HTML details、重い経営エッセイ調",
			TargetLengthStructure: "1800-2400字。前提、実装、コード、確認、まとめ",
			ToneStance:            "明るいチュートリアル。博多弁を少し混ぜ、コードと手順を主役にする",
			SourceSelectors:       []string{"zenn:cloudia"},
			PromptMustContain:     []string{":::message", "topics は5個以内", "クラウディア"},
		},
		{
			ID:                    "cloudia_qiita_how_to",
			PersonaID:             personadomain.IDCloudia,
			OutputFormatID:        outputformat.IDQiitaArticle,
			Medium:                "Qiita",
			Style:                 "practical how-to",
			Theme:                 "Qiita向け記事でdiffコードと注意書きを正しく出す",
			OpeningEpisode:        "Zenn用の:::messageをQiita原稿に混ぜてしまい、レビューで修正が必要になった場面",
			Reader:                "Qiitaに実装メモを投稿するエンジニア",
			ExpectedReaderAction:  "Qiita形式のfrontmatter、diff_language、:::noteを使って再現手順を書く",
			MustInclude:           "環境、手順、diff_go例、:::note warn、確認結果、参考リンク",
			PersonalContext:       "クラウディアとして、試してすぐ動く実用手順に寄せる",
			Exclusions:            "Zennの:::details、note風の長い内省、未検証のベストプラクティス断言",
			TargetLengthStructure: "1400-2000字。環境、手順、コード差分、結果、補足",
			ToneStance:            "実用重視の明るいハウツー。手順を短く区切る",
			SourceSelectors:       []string{"qiita:Cloudia_Cor_Inc"},
			PromptMustContain:     []string{":::note info", "diff_ruby", "Qiita"},
		},
		{
			ID:                    "cor_homepage_section",
			PersonaID:             personadomain.IDTerisuke,
			OutputFormatID:        outputformat.IDHomepageSection,
			Medium:                "homepage",
			Style:                 "concise product section",
			Theme:                 "媒体別の文章生成を事業サイトで短く伝える",
			OpeningEpisode:        "記事生成の検証成果を、トップページの1セクションに圧縮する必要が出た場面",
			Reader:                "Cor.incのサイト訪問者、協業候補、採用候補者",
			ExpectedReaderAction:  "記事生成基盤が実装と発信の両方を支えることを理解し、問い合わせに進む",
			MustInclude:           "媒体別最適化、検証可能な生成、会社サイト向けのCTA",
			PersonalContext:       "Cor.incとして、AI実装と発信支援を同じ品質基準で扱う姿勢",
			Exclusions:            "Markdown、frontmatter、コードブロック、長い説明",
			TargetLengthStructure: "400-700字相当。h2、短い説明、CTA",
			ToneStance:            "落ち着いた会社サイト文体。短く具体的に価値を伝える",
			SourceSelectors: []string{
				"rss:https://cor-jp.com/rss.xml",
				"github:Cor-Incorporated/corsweb2024/src/content/blog/ja",
			},
			PromptMustContain: []string{"<section>", "MarkdownではなくHTML", "CTA"},
		},
	}
}

func casesMarkdown(cases []caseResult) string {
	var builder strings.Builder
	builder.WriteString("# Media matrix scenario\n\n")
	builder.WriteString("Offline planned LLM run matrix. Live source and LLM commands are documented separately and are not run by this scenario.\n\n")
	builder.WriteString("| Case | Persona | Format | Medium | Style | Target length | Sources |\n")
	builder.WriteString("|---|---|---|---|---|---|---|\n")
	for _, item := range cases {
		builder.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | %s | %s | %s | `%s` |\n",
			item.ID,
			item.PersonaID,
			item.OutputFormatID,
			escapeTable(item.Medium),
			escapeTable(item.Style),
			escapeTable(item.TargetLengthStructure),
			strings.Join(item.SourceSelectors, "`, `"),
		))
	}
	builder.WriteString("\n## Planned live LLM commands\n\n")
	for _, item := range cases {
		builder.WriteString("### " + item.ID + "\n\n")
		builder.WriteString("```sh\n" + item.PlannedLLMCommand + "\n```\n\n")
	}
	return builder.String()
}

func escapeTable(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func writeJSON(path string, value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatalf("encode %s: %v", path, err)
	}
	writeFile(path, string(encoded)+"\n")
}

func writeFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fatalf("write %s: %v", path, err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
