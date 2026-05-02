package draft

import (
	"fmt"
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
	return BuildPromptForMode(guide, brief, persona, format)
}

// BuildPromptForMode creates a persona- and format-aware local-LLM instruction.
func BuildPromptForMode(guide WritingStyleGuide, brief ArticleBrief, persona personadomain.Persona, format outputformat.OutputFormat) string {
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

	prompt.WriteString("\n## 出力条件\n")
	appendOutputConditions(&prompt, format.ID)

	return prompt.String()
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
			"文体ガイドの頻出テーマは、無理な羅列にせず、著者の背景や比喩として自然に4語以上回収する。",
			"一人称は指定に従うが、全文で6〜8回程度に抑える。主語を省略できる文では省略し、各段落の冒頭を一人称で始めない。",
			"鉤括弧の内省や印象的な言葉を4〜6箇所ほど入れ、違和感や気づきを残す。",
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
