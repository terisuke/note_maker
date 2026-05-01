package article

import (
	"fmt"
	"strings"

	domain "github.com/teradakousuke/note_maker/internal/domain/article"
)

// BuildPrompt creates the instruction sent to the local LLM.
func BuildPrompt(req domain.GenerationRequest, references []domain.Article) string {
	var prompt strings.Builder

	prompt.WriteString("あなたは、Noteにそのまま貼り付けて公開前編集に入れる完成度の日本語記事を書く編集者です。\n")
	if req.TargetAudience != "" {
		prompt.WriteString(fmt.Sprintf("特に、%s向けの解説を得意としています。\n", req.TargetAudience))
	}
	prompt.WriteString("参考記事から事実、論点、語り口、段落の運び、個人的な経験から抽象化へ進む癖を抽出し、ユーザーの指示を中心に据えた新しい記事として再構成してください。\n")
	prompt.WriteString("参考記事の固有文を写すのではなく、書き手がどこで悩み、どこで違和感を言語化し、どこで読者に橋を架けているかという文章設計を再現してください。\n")
	prompt.WriteString("一人称は参考記事で最も自然に使われているものに揃えてください。参考記事に著者の経歴や繰り返し現れるテーマがある場合は、新しい記事の体験軸として自然に反映してください。\n")
	prompt.WriteString("出力はNoteに貼り付けるMarkdown本文だけにしてください。内部推論、前置き、解説、コードフェンスは出力しないでください。\n\n")

	prompt.WriteString("--- 入力コンテキスト ---\n")
	if len(references) > 0 {
		prompt.WriteString("参考記事:\n")
		for i, article := range references {
			prompt.WriteString(fmt.Sprintf("記事 %d", i+1))
			if article.Title != "" {
				prompt.WriteString(fmt.Sprintf(" - %s", article.Title))
			}
			if article.URL != "" {
				prompt.WriteString(fmt.Sprintf(" (%s)", article.URL))
			}
			prompt.WriteString(fmt.Sprintf(":\n%s\n\n", truncateReference(article.Content)))
		}
	}

	prompt.WriteString("記事作成の指示:\n")
	appendLine(&prompt, "キーワード", strings.Join(req.Keywords, ", "))
	appendLine(&prompt, "テーマ", req.Theme)
	appendLine(&prompt, "想定読者層", req.TargetAudience)
	appendLine(&prompt, "文体", req.StyleChoice)
	appendLine(&prompt, "トーン", req.ToneChoice)
	if req.WordCount > 0 {
		prompt.WriteString(fmt.Sprintf("目標文字数: %d\n", req.WordCount))
	}
	appendLine(&prompt, "記事の目的", req.ArticlePurpose)
	appendLine(&prompt, "含めたい内容", req.DesiredContent)
	appendLine(&prompt, "導入部分のポイント", req.IntroductionPoints)
	appendLine(&prompt, "本論のポイント", req.MainPoints)
	appendLine(&prompt, "結論のメッセージ", req.ConclusionMessage)
	appendLine(&prompt, "含めない内容", req.Exclusions)

	prompt.WriteString("--- 出力指示 ---\n")
	prompt.WriteString("1. 必ず1行目を `# タイトル` 形式にする。タイトル前に文章を置かない。\n")
	prompt.WriteString("2. `##` 見出しを3〜5個使い、導入、本論、結論が自然につながる構成にする。\n")
	prompt.WriteString("3. 参考記事を丸写しせず、ユーザーのテーマ・目的・含めたい内容を記事の主軸にする。\n")
	prompt.WriteString("4. 導入は一般論ではなく、書き手自身の体験、失敗、違和感、問いのいずれかから始める。\n")
	prompt.WriteString("5. 抽象論だけで終わらせず、読者が次に取れる行動や判断基準を入れる。\n")
	prompt.WriteString("6. 参考記事に一人称、会話調の引用、内省、技術や仕事の具体例が多い場合は、その比率に近づける。\n")
	prompt.WriteString("7. 一人称を過剰に連呼せず、体験、事実、読者への問いを交互に置く。\n")
	prompt.WriteString("8. 箇条書きは必要な箇所に限定し、前後に説明文を加える。\n")
	prompt.WriteString("9. 見出し末尾のコロン、過剰な太字、AIらしい定型句を避ける。\n")
	prompt.WriteString("10. 事実として断言できない内容は、断定せず自然な表現で扱う。\n")
	prompt.WriteString("11. 最後は読者への自然な結論で終え、署名、補足説明、生成メモは出さない。\n")
	prompt.WriteString("12. Markdown本文のみを返す。```markdown などのコードフェンスで囲まない。\n")

	return prompt.String()
}

func appendLine(builder *strings.Builder, label, value string) {
	if value == "" {
		return
	}
	builder.WriteString(fmt.Sprintf("%s: %s\n", label, value))
}

func truncateReference(value string) string {
	const maxRunes = 2500
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "\n（参考記事は長いためここで省略）"
}
