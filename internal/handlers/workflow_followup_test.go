package handlers

import (
	"strings"
	"testing"

	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
)

func TestBuildFollowUpPromptIncludesParentContextAndStyleGuide(t *testing.T) {
	session, err := briefdomain.NewArticleBriefSession("session-1", "style-1")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	target := briefdomain.FixedQuestions()[1]
	answer := briefdomain.BriefAnswer{
		QuestionID: target.ID,
		Content:    "会社ブログとして、社員に実装判断の背景まで共有したい",
		FlowType:   briefdomain.QuestionFlowMain,
	}
	prompt := buildFollowUpPrompt(session, target, answer, 1, "- 断定口調\n- 技術知見とビジョン共有を重視")

	for _, want := range []string{
		"親質問: " + target.Text,
		"親回答: 会社ブログとして、社員に実装判断の背景まで共有したい",
		"文体ガイド:",
		"- 技術知見とビジョン共有を重視",
		"質問は必ず「会社ブログとして、社員に実装判断の背景まで共有したい」というご回答を踏まえて、から始める",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestExtractFollowUpQuestionKeepsQuotedRationale(t *testing.T) {
	generated := "承知しました。\n- 「社員に実装判断の背景まで共有したい」というご回答を踏まえて、どの判断を最初に説明しますか？\n"
	got := extractFollowUpQuestion(generated)
	want := "「社員に実装判断の背景まで共有したい」というご回答を踏まえて、どの判断を最初に説明しますか？"
	if got != want {
		t.Fatalf("question = %q, want %q", got, want)
	}
	if !briefdomain.IsAllowedFollowUpQuestion(got) {
		t.Fatalf("extracted contextual question should be allowed: %q", got)
	}
}
