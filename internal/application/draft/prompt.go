package draft

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

const maxGuideRunes = 3000

// BuildPrompt creates the concise local-LLM instruction from the style guide and brief only.
func BuildPrompt(guide WritingStyleGuide, brief ArticleBrief) string {
	persona, _ := personadomain.DefaultRegistry().Get(brief.PersonaID)
	format, ok := outputformat.DefaultRegistry().Get(brief.OutputFormatID)
	if !ok {
		format = outputformat.DefaultRegistry().MustGet(outputformat.IDNoteArticle)
	}
	return BuildPromptForModeWithProfile(guide, brief, AuthorStyleProfile{}, persona, format)
}

// BuildPromptForMode creates a persona- and format-aware local-LLM instruction.
func BuildPromptForMode(guide WritingStyleGuide, brief ArticleBrief, persona personadomain.Persona, format outputformat.OutputFormat) string {
	return BuildPromptForModeWithProfile(guide, brief, AuthorStyleProfile{}, persona, format)
}

// BuildPromptForModeWithProfile creates a persona- and format-aware local-LLM instruction with strict metric hints.
func BuildPromptForModeWithProfile(guide WritingStyleGuide, brief ArticleBrief, profile AuthorStyleProfile, persona personadomain.Persona, format outputformat.OutputFormat) string {
	var prompt strings.Builder

	prompt.WriteString("あなたは指定された人格と媒体に合わせて日本語の下書きを作る編集者です。\n")
	prompt.WriteString("参考記事本文は与えられていません。文体ガイドと記事ブリーフだけを根拠に、新しい記事を書いてください。\n")
	prompt.WriteString("前置き、解説、内部メモは出力しないでください。\n\n")

	prompt.WriteString("## 書き分けモード\n")
	appendLine(&prompt, "Persona", persona.ID+" / "+persona.DisplayName)
	appendLine(&prompt, "OutputFormat", format.ID+" / "+format.DisplayName)
	appendLine(&prompt, "媒体ルール", format.PromptFragment)
	appendLine(&prompt, "人格メモ", persona.PromptHint())
	prompt.WriteString("\n")

	if guideMarkdown := formatGuideMarkdown(format.ID); guideMarkdown != "" {
		prompt.WriteString("## 媒体別Markdownガイド\n")
		prompt.WriteString(guideMarkdown)
		prompt.WriteString("\n\n")
	}

	prompt.WriteString("## 文体ガイド\n")
	prompt.WriteString(truncateRunes(formatStyleGuide(guide), maxGuideRunes))
	prompt.WriteString("\n\n")

	prompt.WriteString("## 記事ブリーフ\n")
	appendLine(&prompt, "テーマ", brief.Theme)
	appendLine(&prompt, "導入エピソード", brief.OpeningEpisode)
	appendLine(&prompt, "読者", brief.Reader)
	appendLine(&prompt, "読後に起こしたい変化", brief.ExpectedReaderAction)
	appendLine(&prompt, "必ず含めること", brief.MustInclude)
	appendLine(&prompt, "著者本人の属人的な文脈", brief.PersonalContext)
	appendLine(&prompt, "含めないこと", brief.Exclusions)
	appendLine(&prompt, "目標文字数と構成", brief.TargetLengthStructure)
	appendLine(&prompt, "トーンと立場", brief.ToneStance)
	if firstPerson := preferredFirstPerson(guide, brief); firstPerson != "" {
		appendLine(&prompt, "一人称", firstPerson)
	}
	appendCustomAnswers(&prompt, brief.CustomAnswers)
	appendDeepDives(&prompt, brief.DeepDives)

	if calibration := strictMetricCalibration(profile, brief, guide); calibration != "" {
		prompt.WriteString("\n## strict style calibration\n")
		prompt.WriteString(calibration)
		prompt.WriteString("\n")
	}

	prompt.WriteString("\n## 出力条件\n")
	appendOutputConditions(&prompt, format.ID)

	return prompt.String()
}

// BuildStyleRevisionPrompt asks the local model for one constrained rewrite when strict evaluation fails.
func BuildStyleRevisionPrompt(originalPrompt, draftMarkdown string, evaluation StyleEvaluation) string {
	var prompt strings.Builder
	prompt.WriteString("以下の下書きは媒体形式としては使えますが、strict style evaluation の一部を満たしていません。\n")
	prompt.WriteString("元の指示、媒体ルール、文体ガイド、記事ブリーフを維持し、失敗した指標だけを改善するように全文を書き直してください。\n")
	prompt.WriteString("前置き、解説、内部メモは出力せず、修正版の下書き本文だけを返してください。\n\n")

	prompt.WriteString("## 元の生成指示\n")
	prompt.WriteString(truncateRunes(originalPrompt, 4500))
	prompt.WriteString("\n\n")

	prompt.WriteString("## strict style evaluation failures\n")
	if len(evaluation.Failures) == 0 {
		prompt.WriteString("- failed without detailed metric\n")
	} else {
		for _, failure := range evaluation.Failures {
			prompt.WriteString("- " + failure + "\n")
		}
	}
	if evaluation.RequiredFirstPerson != "" {
		appendLine(&prompt, "必須一人称", evaluation.RequiredFirstPerson)
	}
	if detail := revisionMetricDetail(evaluation); detail != "" {
		prompt.WriteString(detail)
	}
	prompt.WriteString("評価を上げるために、一人称密度、段落長、文長、引用表現、頻出テーマの自然な回収を調整してください。閾値や媒体形式は変えないでください。\n\n")

	prompt.WriteString("## 現在の下書き\n")
	prompt.WriteString(truncateRunes(draftMarkdown, 6000))
	prompt.WriteString("\n")
	return prompt.String()
}

func revisionMetricDetail(evaluation StyleEvaluation) string {
	var lines []string
	ref := evaluation.Comparison.Reference
	cand := evaluation.Comparison.Candidate
	if ref.CharCount > 0 && cand.CharCount > 0 {
		refFirst := float64(ref.FirstPersonCount) / float64(ref.CharCount) * 1000
		candFirst := float64(cand.FirstPersonCount) / float64(cand.CharCount) * 1000
		firstDirection := "維持"
		if candFirst > refFirst*1.2 {
			firstDirection = "減らす"
		} else if candFirst < refFirst*0.8 {
			firstDirection = "増やす"
		}
		lines = append(lines, fmt.Sprintf("- first_person density: reference %.2f/1000字, current %.2f/1000字。一人称（僕/私/俺の合計）は%s方向で調整", refFirst, candFirst, firstDirection))
		refQuote := float64(ref.QuoteCount) / float64(ref.CharCount) * 1000
		candQuote := float64(cand.QuoteCount) / float64(cand.CharCount) * 1000
		direction := "維持"
		if candQuote > refQuote*1.2 {
			direction = "減らす"
		} else if candQuote < refQuote*0.8 {
			direction = "増やす"
		}
		lines = append(lines, fmt.Sprintf("- quote density: reference %.2f/1000字, current %.2f/1000字。鉤括弧は%s方向で調整", refQuote, candQuote, direction))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func appendOutputConditions(prompt *strings.Builder, formatID string) {
	common := []string{
		"媒体ルールを最優先し、指定された形式以外を混ぜない。",
		"見出しを使い、導入、本論、結論が自然につながる構成にする。",
		"文体ガイドの語り口、段落の長さ、具体例の配分を再現する。",
		"ブリーフの必須要素を本文に自然に入れ、除外事項は扱わない。",
		"指定された目標文字数に近づける。短い要約で終わらせず、各見出しで十分に展開する。",
		"出力全体をコードフェンスで囲まない。",
	}
	formatSpecific := map[string][]string{
		outputformat.IDNoteArticle: {
			"noteで変換されやすい最小記法だけを使う。##、###、>、-、1.、**太字**、~~取り消し線~~、---、通常のコードブロックまでに抑える。",
			"表、脚注、数式、HTML、Zenn/Qiita独自の補足ブロック、ファイル名付きコードフェンスは使わない。",
			"文体ガイドとstrict style calibrationの頻出テーマは、無理な羅列にせず、著者の背景や比喩として自然に10語以上回収する。",
			"一人称は指定とstrict style calibrationの目標密度に従う。主語を省略できる文では省略し、各段落の冒頭を一人称で始めない。",
			"鉤括弧の内省や印象的な言葉はstrict style calibrationの目標密度に従い、短い引用を乱発しない。",
			"各段落は短すぎる断片にせず、体験、解釈、読者への接続をできるだけ一段落内で結ぶ。",
			"最後は読者に残す判断基準か次の一歩で締める。",
		},
		outputformat.IDMarkdownBlog: {
			"frontmatterのcategoryは ai / engineering / founder / lab のいずれかを選び、langは必ず ja にする。",
			"会社の技術的知見、実装判断、検証結果、社員へのビジョン共有のいずれに資する記事かが本文から分かるようにする。",
			"敬語より断定口調を優先し、「実装する」「分かった」「重要だ」のように端的に書く。",
			"コードフェンスを使う場合は必ず言語名を付け、読者が再現できる前提条件と確認結果を書く。",
			"最後は会社として次にどう活かすか、または読者が業務でどう判断すべきかで締める。",
		},
		outputformat.IDZennArticle: {
			"技術的前提、手順、コード、失敗しやすい点、参考リンクを分け、読者が再現できる構成にする。",
			"topicsは5個以内の短い技術タグにする。",
			"コードブロックは言語名、必要なら `言語:ファイル名`、差分なら `diff 言語` を使う。",
			"補足は :::message、注意は :::message alert、折りたたみは :::details、リンクカードは @[card](URL) を使う。",
			"Qiitaの :::note、diff_language、HTML details を混ぜない。",
			"キャラクター口調は入れすぎず、技術説明の明快さを優先する。",
		},
		outputformat.IDQiitaArticle: {
			"環境、手順、コード、実行結果、トラブルシュートを明確にし、読者がすぐ試せる粒度にする。",
			"tagsはQiitaで検索されやすい技術名を中心にする。",
			"コードブロックは言語名、必要なら `言語:ファイル名`、差分なら `diff_言語` を使う。",
			"補足・警告は :::note info / :::note warn / :::note alert、折りたたみは <details><summary>...</summary>...</details> を使う。",
			"数式は ```math ブロック、インライン数式は $`...`$ を優先し、Zennの :::message や :::details を混ぜない。",
		},
		outputformat.IDHomepageSection: {
			"MarkdownではなくHTMLだけを出力し、会社サイトでそのまま埋め込める密度にする。",
			"本文は機能説明だけでなく、読者が問い合わせや相談に進む理由を含める。",
		},
	}
	lines := append([]string{}, common...)
	lines = append(lines, formatSpecific[formatID]...)
	for i, line := range lines {
		prompt.WriteString(fmt.Sprintf("%d. %s\n", i+1, line))
	}
}

func strictMetricCalibration(profile AuthorStyleProfile, brief ArticleBrief, guide WritingStyleGuide) string {
	if profile.Metrics.CharCount <= 0 || profile.Metrics.FirstPersonCount <= 0 {
		return ""
	}
	firstPerson := preferredFirstPerson(guide, brief)
	if firstPerson == "" {
		firstPerson = normalizedFirstPerson(profile.PreferredFirstPerson)
	}
	if firstPerson == "" {
		return ""
	}
	targetRunes := targetRunesFromBrief(brief.TargetLengthStructure)
	density := float64(profile.Metrics.FirstPersonCount) / float64(profile.Metrics.CharCount)
	targetCount := int(math.Round(density * float64(targetRunes)))
	if targetCount < 1 {
		targetCount = 1
	}
	lower := maxInt(1, int(math.Floor(float64(targetCount)*0.8)))
	upper := maxInt(lower, int(math.Ceil(float64(targetCount)*1.2)))
	lines := []string{fmt.Sprintf(
		"参照文体の一人称密度は約 %.2f 回/1000字です。今回の目標長さでは一人称「%s」を中心にし、「僕」「私」「俺」の合計を%d〜%d回程度に収めてください。指定がない限り「%s」以外の一人称を混ぜないでください。少なすぎても多すぎてもfirst_person評価が下がります。\n",
		density*1000,
		firstPerson,
		lower,
		upper,
		firstPerson,
	)}
	if quoteLine := quoteDensityCalibration(profile, targetRunes); quoteLine != "" {
		lines = append(lines, quoteLine)
	}
	if keywordLine := keywordCalibration(profile); keywordLine != "" {
		lines = append(lines, keywordLine)
	}
	return strings.Join(lines, "")
}

func quoteDensityCalibration(profile AuthorStyleProfile, targetRunes int) string {
	if profile.Metrics.CharCount <= 0 || profile.Metrics.QuoteCount <= 0 {
		return ""
	}
	density := float64(profile.Metrics.QuoteCount) / float64(profile.Metrics.CharCount)
	targetCount := int(math.Round(density * float64(targetRunes)))
	if targetCount < 1 {
		targetCount = 1
	}
	lower := maxInt(1, int(math.Floor(float64(targetCount)*0.8)))
	upper := maxInt(lower, int(math.Ceil(float64(targetCount)*1.2)))
	return fmt.Sprintf(
		"参照文体の鉤括弧密度は約 %.2f 個/1000字です。今回の目標長さでは開始記号「 または『 を合計%d〜%d個程度に収めてください。短い引用を増やしすぎるとquote_density評価が下がります。\n",
		density*1000,
		lower,
		upper,
	)
}

func keywordCalibration(profile AuthorStyleProfile) string {
	if len(profile.Metrics.KeywordCounts) == 0 {
		return ""
	}
	keywords := make([]string, 0, len(profile.Metrics.KeywordCounts))
	for keyword, count := range profile.Metrics.KeywordCounts {
		if count > 0 {
			keywords = append(keywords, keyword)
		}
	}
	if len(keywords) == 0 {
		return ""
	}
	sortStrings(keywords)
	return fmt.Sprintf(
		"参照文体の主要キーワード候補: %s。本文ではこのうち少なくとも10語を、不自然な羅列ではなく体験・比喩・判断基準の中で自然に回収してください。\n",
		strings.Join(keywords, " / "),
	)
}

func sortStrings(values []string) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

func targetRunesFromBrief(value string) int {
	const fallback = 3000
	matches := regexp.MustCompile(`([0-9]{3,5})\s*字`).FindStringSubmatch(value)
	if len(matches) != 2 {
		return fallback
	}
	parsed, err := strconv.Atoi(matches[1])
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatStyleGuide(guide WritingStyleGuide) string {
	var builder strings.Builder
	appendLine(&builder, "文体分析メモ", guide.Markdown)
	appendLine(&builder, "一人称", guide.PreferredFirstPerson)
	appendList(&builder, "繰り返すテーマ", guide.RecurringThemes)
	appendLine(&builder, "段落リズム", guide.ParagraphRhythm)
	appendLine(&builder, "文のリズム", guide.SentenceRhythm)
	appendLine(&builder, "見出し", guide.HeadingGuidance)
	appendLine(&builder, "引用表現", guide.QuoteGuidance)
	appendList(&builder, "導入パターン", guide.OpeningPatterns)
	appendList(&builder, "結論パターン", guide.ConclusionPatterns)
	appendList(&builder, "注意点", guide.Warnings)
	return strings.TrimSpace(builder.String())
}

func appendLine(builder *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	builder.WriteString(fmt.Sprintf("%s: %s\n", label, value))
}

func appendList(builder *strings.Builder, label string, values []string) {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			cleaned = append(cleaned, value)
		}
	}
	if len(cleaned) == 0 {
		return
	}
	builder.WriteString(label + ":\n")
	for _, value := range cleaned {
		builder.WriteString("- " + value + "\n")
	}
}

func appendDeepDives(builder *strings.Builder, values []BriefAnswer) {
	lines := make([]string, 0, len(values))
	for _, value := range values {
		content := strings.TrimSpace(value.Content)
		if content != "" {
			lines = append(lines, content)
		}
	}
	appendList(builder, "深掘りメモ", lines)
}

func appendCustomAnswers(builder *strings.Builder, values []BriefAnswer) {
	lines := make([]string, 0, len(values))
	for _, value := range values {
		content := strings.TrimSpace(value.Content)
		if content != "" {
			lines = append(lines, content)
		}
	}
	appendList(builder, "追加質問メモ", lines)
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "\n（文体ガイドは長いためここで省略）"
}

func preferredFirstPerson(guide WritingStyleGuide, brief ArticleBrief) string {
	if override := explicitFirstPersonOverride(brief); override != "" {
		return override
	}
	return normalizedFirstPerson(guide.PreferredFirstPerson)
}

func explicitFirstPersonOverride(brief ArticleBrief) string {
	text := strings.Join([]string{brief.ToneStance, brief.MustInclude, brief.Exclusions}, "\n")
	if !strings.Contains(text, "一人称") {
		return ""
	}
	for _, candidate := range []string{"僕", "私", "俺", "自分", "クラウディア", "うち"} {
		if strings.Contains(text, candidate) {
			return candidate
		}
	}
	return ""
}

func normalizedFirstPerson(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "明示しない" {
		return ""
	}
	return value
}
