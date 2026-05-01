package author

import (
	"fmt"
	"strings"
)

// BuildWritingStyleGuide derives compact, editable guidance from an author profile.
func BuildWritingStyleGuide(profile AuthorStyleProfile) (WritingStyleGuide, error) {
	if err := profile.Validate(); err != nil {
		return WritingStyleGuide{}, err
	}
	guide := WritingStyleGuide{
		ID:                   GuideID(profile),
		ProfileID:            profile.ID,
		PreferredFirstPerson: profile.PreferredFirstPerson,
		RecurringThemes:      append([]string(nil), profile.RecurringKeywords...),
		ParagraphRhythm:      paragraphRhythm(profile.Metrics.AverageParagraphRunes),
		SentenceRhythm:       sentenceRhythm(profile.Metrics.AverageSentenceRunes),
		HeadingGuidance:      headingGuidance(profile.Metrics.HeadingCount, profile.ArticleCount),
		QuoteGuidance:        quoteGuidance(profile.Metrics.QuoteCount, profile.Metrics.CharCount),
		OpeningPatterns:      openingPatterns(profile),
		ConclusionPatterns:   conclusionPatterns(profile),
		Warnings:             guideWarnings(profile),
	}
	guide.Markdown = GuideMarkdown(guide)
	if err := guide.Validate(); err != nil {
		return WritingStyleGuide{}, err
	}
	return guide, nil
}

// GuideMarkdown renders the structured guide into compact Markdown for prompt builders.
func GuideMarkdown(guide WritingStyleGuide) string {
	var builder strings.Builder
	appendGuideLine(&builder, "一人称", guide.PreferredFirstPerson)
	appendGuideList(&builder, "よく扱うテーマ", guide.RecurringThemes)
	appendGuideLine(&builder, "段落のリズム", guide.ParagraphRhythm)
	appendGuideLine(&builder, "文のリズム", guide.SentenceRhythm)
	appendGuideLine(&builder, "見出し", guide.HeadingGuidance)
	appendGuideLine(&builder, "引用表現", guide.QuoteGuidance)
	appendGuideList(&builder, "書き出し", guide.OpeningPatterns)
	appendGuideList(&builder, "締め方", guide.ConclusionPatterns)
	appendGuideList(&builder, "注意点", guide.Warnings)
	return strings.TrimSpace(builder.String())
}

// MustBuildWritingStyleGuide is a convenience helper for tests and static fixtures.
func MustBuildWritingStyleGuide(profile AuthorStyleProfile) WritingStyleGuide {
	guide, err := BuildWritingStyleGuide(profile)
	if err != nil {
		panic(err)
	}
	return guide
}

func appendGuideLine(builder *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	builder.WriteString("- " + label + ": " + value + "\n")
}

func appendGuideList(builder *strings.Builder, label string, values []string) {
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
	builder.WriteString("- " + label + ": " + strings.Join(cleaned, " / ") + "\n")
}

func paragraphRhythm(averageRunes float64) string {
	switch {
	case averageRunes >= 180:
		return "長めの段落で経験や理由をまとめて展開する"
	case averageRunes >= 90:
		return "中くらいの段落で具体例と解釈を交互に置く"
	default:
		return "短い段落で余白を作りながらテンポよく進める"
	}
}

func sentenceRhythm(averageRunes float64) string {
	switch {
	case averageRunes >= 70:
		return "一文はやや長めで、背景から結論までを接続して書く"
	case averageRunes >= 35:
		return "一文は中程度で、主張と補足を自然につなぐ"
	default:
		return "短い文を重ねて、読み手が追いやすいリズムにする"
	}
}

func headingGuidance(headings, articleCount int) string {
	if articleCount <= 0 {
		return "必要に応じてMarkdown見出しで流れを分ける"
	}
	averageHeadings := float64(headings) / float64(articleCount)
	switch {
	case averageHeadings >= 3:
		return "Markdown見出しを複数置き、話題の転換を明確にする"
	case averageHeadings >= 1:
		return "主要な転換点にだけMarkdown見出しを置く"
	default:
		return "見出しに頼りすぎず、段落の流れで読ませる"
	}
}

func quoteGuidance(quoteCount, charCount int) string {
	if charCount <= 0 {
		return "引用表現は必要な場面に限る"
	}
	density := float64(quoteCount) / float64(charCount)
	switch {
	case density >= 0.015:
		return "印象的な言葉や内省を鉤括弧で残す"
	case density > 0:
		return "要所で鉤括弧を使い、違和感や気づきを強調する"
	default:
		return "鉤括弧は無理に増やさず、自然な説明を優先する"
	}
}

func openingPatterns(profile AuthorStyleProfile) []string {
	firstPerson := profile.PreferredFirstPerson
	if strings.TrimSpace(firstPerson) == "" || firstPerson == "明示しない" {
		firstPerson = "自分"
	}
	return []string{
		fmt.Sprintf("%sの具体的な体験や違和感から書き始める", firstPerson),
		"読者が状況を想像できる小さな場面を先に置く",
	}
}

func conclusionPatterns(profile AuthorStyleProfile) []string {
	themes := profile.RecurringKeywords
	if len(themes) == 0 {
		themes = []string{"学び"}
	}
	return []string{
		fmt.Sprintf("最後は%sにつながる学びとして回収する", themes[0]),
		"読者が次に試せる小さな行動で締める",
	}
}

func guideWarnings(profile AuthorStyleProfile) []string {
	warnings := append([]string(nil), profile.Warnings...)
	if profile.PreferredFirstPerson == "明示しない" {
		warnings = append(warnings, "preferred_first_person_unclear")
	}
	if len(profile.RecurringKeywords) < 3 {
		warnings = append(warnings, "recurring_theme_count_low")
	}
	return warnings
}
