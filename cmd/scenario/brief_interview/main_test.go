package main

import (
	"testing"

	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
)

func TestScenarioAnswersMapFixedQuestionsByID(t *testing.T) {
	answers := scenarioAnswers("note_reflection")

	for _, question := range briefdomain.FixedQuestions() {
		answer, ok := answers[question.ID]
		if !ok {
			t.Fatalf("missing scripted answer for fixed question %q", question.ID)
		}
		if answer == "" {
			t.Fatalf("empty scripted answer for fixed question %q", question.ID)
		}
	}

	if got := answers[briefdomain.QuestionIDMustInclude]; got != "Note APIで記事を集めること、文体ガイドと一問一答を分けること、深掘り質問で記事の核を作ること。音楽家からエンジニア、起業、LT登壇、AI駆動開発という自分の文脈も自然に接続したい" {
		t.Fatalf("must_include shifted or changed: %q", got)
	}
	if got := answers[briefdomain.QuestionIDExclusions]; got != "ローカルLLMを万能だと断言すること、根拠のない性能比較、Gemini依存" {
		t.Fatalf("exclusions shifted or changed: %q", got)
	}
	if got := answers[briefdomain.QuestionIDTargetLengthStructure]; got != "3000字前後、最低2800字。導入、違和感、設計変更、Evo X2でのモデル使い分け、実装と検証、読者への提案、結論で構成する" {
		t.Fatalf("target length shifted or changed: %q", got)
	}
}
