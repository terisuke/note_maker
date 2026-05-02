package draft

import (
	"fmt"
	"strings"
)

const maxGuideRunes = 3000

// BuildPrompt creates the concise local-LLM instruction from the style guide and brief only.
func BuildPrompt(guide WritingStyleGuide, brief ArticleBrief) string {
	var prompt strings.Builder

	prompt.WriteString("あなたはNoteにそのまま貼り付けられる日本語Markdown記事を書く編集者です。\n")
	prompt.WriteString("参考記事本文は与えられていません。文体ガイドと記事ブリーフだけを根拠に、新しい記事を書いてください。\n")
	prompt.WriteString("出力はMarkdown本文のみ。前置き、解説、内部メモ、コードフェンスは出力しないでください。\n\n")

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
	prompt.WriteString("1. 1行目は必ず `# タイトル` 形式にする。\n")
	prompt.WriteString("2. `##` 見出しを使い、導入、本論、結論が自然につながる構成にする。\n")
	prompt.WriteString("3. 文体ガイドの語り口、段落の長さ、内省と具体例の配分を再現する。\n")
	prompt.WriteString("4. ブリーフの必須要素を本文に自然に入れ、除外事項は扱わない。\n")
	prompt.WriteString("5. 文体ガイドの頻出テーマは、無理な羅列にせず、著者の背景や比喩として自然に4語以上回収する。\n")
	prompt.WriteString("6. 一人称は指定に従うが、全文で6〜8回程度に抑える。主語を省略できる文では省略し、各段落の冒頭を一人称で始めない。\n")
	prompt.WriteString("7. 鉤括弧の内省や印象的な言葉を4〜6箇所ほど入れ、違和感や気づきを残す。\n")
	prompt.WriteString("8. 各段落は短すぎる断片にせず、体験、解釈、読者への接続をできるだけ一段落内で結ぶ。\n")
	prompt.WriteString("9. 最後は読者に残す判断基準か次の一歩で締める。\n")
	prompt.WriteString("10. 指定された目標文字数に近づける。3000字指定の場合は最低2800字を目安に、短い要約で終わらせず、各見出しで十分に展開する。\n")
	prompt.WriteString("11. Markdown本文だけを返し、コードフェンスで囲まない。\n")

	return prompt.String()
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
	for _, candidate := range []string{"僕", "私", "俺", "自分"} {
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
