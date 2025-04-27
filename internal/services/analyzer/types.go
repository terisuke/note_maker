package analyzer

// StyleAnalysis は記事の文体分析結果を表す
type StyleAnalysis struct {
	ParagraphLength       string   // 段落の長さ（short, medium, long）
	AverageSentenceLength float64  // 平均文の長さ
	Tone                  string   // 文体のトーン（casual, polite, formal）
	CommonExpressions     []string // よく使われる表現
}

// StyleComparison は2つの記事の文体比較結果を表す
type StyleComparison struct {
	Article1 *StyleAnalysis
	Article2 *StyleAnalysis
}
