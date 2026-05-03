package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	briefapp "github.com/teradakousuke/note_maker/internal/application/brief"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

const defaultOutputDir = "tmp/interview_template"

type scenarioReport struct {
	GeneratedBy            string            `json:"generated_by"`
	OfflineOnly            bool              `json:"offline_only"`
	Cases                  []caseResult      `json:"cases"`
	RequiredBriefFields    []string          `json:"required_brief_fields"`
	ExpectedDeepDiveTarget []string          `json:"expected_deep_dive_target_order"`
	QuestionCoverage       map[string]int    `json:"question_coverage"`
	BriefCoverage          map[string]int    `json:"brief_coverage"`
	Artifacts              map[string]string `json:"artifacts"`
}

type casePlan struct {
	ID             string
	PersonaID      string
	OutputFormatID string
}

type caseResult struct {
	ID                   string          `json:"id"`
	PersonaID            string          `json:"persona_id"`
	PersonaDisplayName   string          `json:"persona_display_name"`
	OutputFormatID       string          `json:"output_format_id"`
	OutputFormatName     string          `json:"output_format_name"`
	SessionID            string          `json:"session_id"`
	QuestionCount        int             `json:"question_count"`
	RequiredQuestionIDs  []string        `json:"required_question_ids"`
	OptionalQuestionIDs  []string        `json:"optional_question_ids"`
	ExtensionQuestionIDs []string        `json:"extension_question_ids"`
	CustomAnswerIDs      []string        `json:"custom_answer_ids"`
	DeepDiveTargetIDs    []string        `json:"deep_dive_target_ids"`
	DeepDiveCount        int             `json:"deep_dive_count"`
	TemplateChecks       []checkResult   `json:"template_checks"`
	BriefChecks          []checkResult   `json:"brief_checks"`
	QuestionTemplate     []questionEntry `json:"question_template"`
	BriefPath            string          `json:"brief_path"`
	SessionPath          string          `json:"session_path"`
}

type questionEntry struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	Required    bool   `json:"required"`
	TargetField string `json:"target_field"`
}

type checkResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

func main() {
	outputDir := envOrDefault("SCENARIO_OUTPUT_DIR", defaultOutputDir)
	report, err := runScenario(context.Background(), outputDir)
	if err != nil {
		fatalf("%v", err)
	}

	fmt.Printf("interview template scenario completed\n")
	fmt.Printf("offline_only=%v\n", report.OfflineOnly)
	fmt.Printf("cases=%d\n", len(report.Cases))
	fmt.Printf("question_templates=%d\n", report.QuestionCoverage["cases"])
	fmt.Printf("briefs=%d\n", report.BriefCoverage["cases"])
	fmt.Printf("report=%s\n", report.Artifacts["report"])
	fmt.Printf("cases_markdown=%s\n", report.Artifacts["cases_markdown"])
}

func runScenario(ctx context.Context, outputDir string) (scenarioReport, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		outputDir = defaultOutputDir
	}
	briefDir := filepath.Join(outputDir, "briefs")
	sessionDir := filepath.Join(outputDir, "sessions")
	for _, dir := range []string{outputDir, briefDir, sessionDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return scenarioReport{}, fmt.Errorf("create output dir %s: %w", dir, err)
		}
	}

	personas := personadomain.DefaultRegistry()
	formats := outputformat.DefaultRegistry()
	service := briefapp.NewInterviewService(nil)

	results := make([]caseResult, 0)
	for _, plan := range scenarioPlans(personas.List(), formats.List()) {
		result, err := runCase(ctx, service, personas, formats, plan, briefDir, sessionDir)
		if err != nil {
			return scenarioReport{}, err
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		return scenarioReport{}, fmt.Errorf("no interview template cases were generated")
	}

	reportPath := filepath.Join(outputDir, "report.json")
	casesPath := filepath.Join(outputDir, "cases.md")
	report := scenarioReport{
		GeneratedBy:            "cmd/scenario/interview_template",
		OfflineOnly:            true,
		Cases:                  results,
		RequiredBriefFields:    requiredBriefFields(),
		ExpectedDeepDiveTarget: expectedDeepDiveTargets(),
		QuestionCoverage:       questionCoverage(results),
		BriefCoverage:          briefCoverage(results),
		Artifacts: map[string]string{
			"report":         reportPath,
			"cases_markdown": casesPath,
			"briefs":         briefDir,
			"sessions":       sessionDir,
		},
	}
	if err := writeJSON(reportPath, report); err != nil {
		return scenarioReport{}, err
	}
	if err := writeFile(casesPath, casesMarkdown(report)); err != nil {
		return scenarioReport{}, err
	}
	return report, nil
}

func scenarioPlans(personas []personadomain.Persona, formats []outputformat.OutputFormat) []casePlan {
	plans := make([]casePlan, 0, len(personas)*len(formats))
	for _, persona := range personas {
		for _, format := range formats {
			plans = append(plans, casePlan{
				ID:             persona.ID + "_" + format.ID,
				PersonaID:      persona.ID,
				OutputFormatID: format.ID,
			})
		}
	}
	return plans
}

func runCase(ctx context.Context, service *briefapp.InterviewService, personas personadomain.Registry, formats outputformat.Registry, plan casePlan, briefDir, sessionDir string) (caseResult, error) {
	persona, ok := personas.Get(plan.PersonaID)
	if !ok {
		return caseResult{}, fmt.Errorf("%s references unknown persona %s", plan.ID, plan.PersonaID)
	}
	format, ok := formats.Get(plan.OutputFormatID)
	if !ok {
		return caseResult{}, fmt.Errorf("%s references unknown output format %s", plan.ID, plan.OutputFormatID)
	}
	sessionID := "interview_template_" + plan.ID
	result, err := service.StartSession(briefapp.StartSessionInput{
		SessionID:      sessionID,
		StyleProfileID: "style_interview_template",
		PersonaID:      plan.PersonaID,
		OutputFormatID: plan.OutputFormatID,
	})
	if err != nil {
		return caseResult{}, fmt.Errorf("%s start session: %w", plan.ID, err)
	}

	for !result.Completed {
		if result.NextQuestion == nil {
			return caseResult{}, fmt.Errorf("%s has no next question before completion", plan.ID)
		}
		question := *result.NextQuestion
		answer := scriptedAnswer(plan, persona, format, question)
		result, err = service.Answer(ctx, result.Session, answer)
		if err != nil {
			return caseResult{}, fmt.Errorf("%s answer %s: %w", plan.ID, question.ID, err)
		}
	}
	if result.Brief == nil {
		return caseResult{}, fmt.Errorf("%s completed without an article brief", plan.ID)
	}

	briefPath := filepath.Join(briefDir, plan.ID+".json")
	sessionPath := filepath.Join(sessionDir, plan.ID+".json")
	if err := writeJSON(briefPath, result.Brief); err != nil {
		return caseResult{}, err
	}
	if err := writeJSON(sessionPath, result.Session); err != nil {
		return caseResult{}, err
	}

	requiredIDs, optionalIDs := splitRequiredQuestionIDs(result.Session.Questions)
	customIDs := customAnswerIDs(result.Brief.CustomAnswers)
	deepDiveTargets := deepDiveTargetIDs(result.Brief.DeepDives)
	return caseResult{
		ID:                   plan.ID,
		PersonaID:            persona.ID,
		PersonaDisplayName:   persona.DisplayName,
		OutputFormatID:       format.ID,
		OutputFormatName:     format.DisplayName,
		SessionID:            result.Session.ID,
		QuestionCount:        len(result.Session.Questions),
		RequiredQuestionIDs:  requiredIDs,
		OptionalQuestionIDs:  optionalIDs,
		ExtensionQuestionIDs: extensionQuestionIDs(result.Session.Questions),
		CustomAnswerIDs:      customIDs,
		DeepDiveTargetIDs:    deepDiveTargets,
		DeepDiveCount:        len(result.Brief.DeepDives),
		TemplateChecks:       templateChecks(plan, result.Session.Questions),
		BriefChecks:          briefChecks(plan, *result.Brief, customIDs, deepDiveTargets),
		QuestionTemplate:     questionTemplate(result.Session.Questions),
		BriefPath:            briefPath,
		SessionPath:          sessionPath,
	}, nil
}

func templateChecks(plan casePlan, questions []briefdomain.ArticleQuestion) []checkResult {
	checks := []checkResult{
		check("unique_question_ids", uniqueQuestionIDs(questions), ""),
		check("base_questions_present", containsAllQuestionIDs(questions, baseQuestionIDs()), ""),
	}
	for _, id := range expectedExtensionQuestionIDs(plan.PersonaID, plan.OutputFormatID) {
		checks = append(checks, check("extension_"+id, containsQuestionID(questions, id), ""))
	}
	return checks
}

func briefChecks(plan casePlan, brief briefdomain.ArticleBrief, customIDs, deepDiveTargets []string) []checkResult {
	checks := []checkResult{
		check("persona_id", brief.PersonaID == plan.PersonaID, brief.PersonaID),
		check("output_format_id", brief.OutputFormatID == plan.OutputFormatID, brief.OutputFormatID),
		check("required_fields", hasRequiredBriefFields(brief), ""),
		check("deep_dive_count", len(brief.DeepDives) == briefdomain.MaxTotalFollowUps, fmt.Sprintf("%d", len(brief.DeepDives))),
		check("deep_dive_targets", strings.Join(deepDiveTargets, ",") == strings.Join(expectedDeepDiveTargets(), ","), strings.Join(deepDiveTargets, ",")),
	}
	for _, id := range expectedExtensionQuestionIDs(plan.PersonaID, plan.OutputFormatID) {
		checks = append(checks, check("custom_answer_"+id, containsString(customIDs, id), ""))
	}
	return checks
}

func scriptedAnswer(plan casePlan, persona personadomain.Persona, format outputformat.OutputFormat, question briefdomain.ArticleQuestion) string {
	if question.FlowType == briefdomain.QuestionFlowDeepDiveFollowUp {
		return deepDiveAnswer(question)
	}
	switch question.ID {
	case briefdomain.QuestionIDTheme:
		return fmt.Sprintf("%s向けに%sの発信テンプレートを検証する", format.DisplayName, persona.DisplayName)
	case briefdomain.QuestionIDOpeningEpisode:
		return "ライブ生成の前に、質問テンプレートだけを固定入力で確認した場面から始める"
	case briefdomain.QuestionIDReader:
		return "Evo X2で媒体別の記事生成を試す開発者とレビュー担当者"
	case briefdomain.QuestionIDReaderProblem:
		return "生成前に、媒体や人格ごとの聞き取り内容が混ざっていないか判断しづらい"
	case briefdomain.QuestionIDExpectedReaderAction:
		return "ライブ下書き生成の前に、このオフラインシナリオを必ず通す"
	case briefdomain.QuestionIDKeyTakeaway:
		return "テンプレートとブリーフを先に固定すると、LLM評価の前提が揃う"
	case briefdomain.QuestionIDMustInclude:
		return "ペルソナID、出力形式ID、追加質問、深掘り回答、完成ArticleBriefの保存先"
	case briefdomain.QuestionIDConcreteExample:
		return "てりすけのnote、クラウディアのZenn、会社ブログ、Qiita、HTMLセクションを同じ規則で確認する"
	case briefdomain.QuestionIDEvidence:
		return "10ケース、全ケース4件の深掘り、必須フィールドと追加回答の存在をJSONで記録する"
	case briefdomain.QuestionIDPersonalContext:
		return "媒体別のライブ実測に入る前に、入力条件の揺れをなくしたい"
	case briefdomain.QuestionIDExclusions:
		return "ネットワークアクセス、LLM呼び出し、実下書き生成、実測値の比較"
	case briefdomain.QuestionIDTargetLengthStructure:
		return "1200字相当。導入、確認対象、ケース、検証結果、次のライブ実行条件で構成する"
	case briefdomain.QuestionIDToneStance:
		return persona.PromptHint()
	case briefdomain.QuestionIDTitleKeywords:
		return "interview template, ArticleBrief, Evo X2 preflight"
	case briefdomain.QuestionIDStoryArc:
		return "違和感から始め、固定入力で確認し、ライブ生成へ進む順番にする"
	case briefdomain.QuestionIDTargetStack:
		return "Go 1.23、cmd/scenario/interview_template、internal/domain/brief"
	case briefdomain.QuestionIDPrerequisiteKnowledge:
		return "GoのテストとJSON成果物を読める開発者を前提にする"
	case briefdomain.QuestionIDTechnicalProof:
		return "go testとgo runの成功、report.jsonのチェック結果を根拠にする"
	case briefdomain.QuestionIDCodeExamples:
		return "必要ならgo run ./cmd/scenario/interview_templateの実行例だけ載せる"
	case briefdomain.QuestionIDReferences:
		return "docs/validation/issue-70-interview-template-scenario-2026-05-03.md"
	case briefdomain.QuestionIDCorBlogPurpose:
		return "技術知見の報告として、ライブ測定の前提条件を社内外に共有する"
	case briefdomain.QuestionIDCorBlogNextAction:
		return "ライブ媒体マトリクスを実行する前にオフラインpreflightを確認してほしい"
	case briefdomain.QuestionIDHomepageCTA:
		return "検証済みブリーフを確認してからライブ生成へ進む"
	case briefdomain.QuestionIDHomepageTrust:
		return "全ケースをオフラインで再現でき、LLMやネットワークに依存しない"
	case briefdomain.QuestionIDCloudiaViewpoint:
		return "媒体ごとの質問が切り替わる様子を、初心者にも楽しく見える確認として扱う"
	default:
		return "この質問はシナリオの固定回答で検証する"
	}
}

func deepDiveAnswer(question briefdomain.ArticleQuestion) string {
	switch question.TargetQuestionID {
	case briefdomain.QuestionIDOpeningEpisode:
		if question.FollowUpIndex == 1 {
			return "最初に見せたいのは、ライブ実行前でも全ケースの質問とブリーフが揃う画面"
		}
		return "その時の気持ちは、測る前に入力を固められて安心したという感覚"
	case briefdomain.QuestionIDMustInclude:
		if question.FollowUpIndex == 1 {
			return "特に詳しく説明したいのは、追加質問がCustomAnswersへ残ること"
		}
		return "根拠としてreport.jsonと各brief JSONを残す"
	default:
		return "記事に足す具体情報として、オフラインで再実行できるコマンドを入れる"
	}
}

func expectedExtensionQuestionIDs(personaID, formatID string) []string {
	ids := make([]string, 0)
	switch formatID {
	case outputformat.IDNoteArticle:
		ids = append(ids, briefdomain.QuestionIDStoryArc)
	case outputformat.IDMarkdownBlog:
		ids = append(ids, technicalQuestionIDs()...)
		ids = append(ids, briefdomain.QuestionIDCorBlogPurpose, briefdomain.QuestionIDCorBlogNextAction)
	case outputformat.IDZennArticle, outputformat.IDQiitaArticle:
		ids = append(ids, technicalQuestionIDs()...)
	case outputformat.IDHomepageSection:
		ids = append(ids, briefdomain.QuestionIDHomepageCTA, briefdomain.QuestionIDHomepageTrust)
	}
	if personaID == personadomain.IDCloudia {
		ids = append(ids, briefdomain.QuestionIDCloudiaViewpoint)
	}
	return ids
}

func technicalQuestionIDs() []string {
	return []string{
		briefdomain.QuestionIDTargetStack,
		briefdomain.QuestionIDPrerequisiteKnowledge,
		briefdomain.QuestionIDTechnicalProof,
		briefdomain.QuestionIDCodeExamples,
		briefdomain.QuestionIDReferences,
	}
}

func baseQuestionIDs() []string {
	questions := briefdomain.FixedQuestions()
	ids := make([]string, 0, len(questions))
	for _, question := range questions {
		ids = append(ids, question.ID)
	}
	return ids
}

func expectedDeepDiveTargets() []string {
	return []string{
		briefdomain.QuestionIDOpeningEpisode,
		briefdomain.QuestionIDOpeningEpisode,
		briefdomain.QuestionIDMustInclude,
		briefdomain.QuestionIDMustInclude,
	}
}

func requiredBriefFields() []string {
	return []string{
		"style_profile_id",
		"persona_id",
		"output_format_id",
		"theme",
		"opening_episode",
		"reader",
		"expected_reader_action",
		"must_include",
		"personal_context",
		"target_length_structure",
		"tone_stance",
	}
}

func hasRequiredBriefFields(brief briefdomain.ArticleBrief) bool {
	values := []string{
		brief.StyleProfileID,
		brief.PersonaID,
		brief.OutputFormatID,
		brief.Theme,
		brief.OpeningEpisode,
		brief.Reader,
		brief.ExpectedReaderAction,
		brief.MustInclude,
		brief.PersonalContext,
		brief.TargetLengthStructure,
		brief.ToneStance,
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func splitRequiredQuestionIDs(questions []briefdomain.ArticleQuestion) ([]string, []string) {
	required := make([]string, 0)
	optional := make([]string, 0)
	for _, question := range questions {
		if question.FlowType != briefdomain.QuestionFlowMain {
			continue
		}
		if question.Required {
			required = append(required, question.ID)
			continue
		}
		optional = append(optional, question.ID)
	}
	return required, optional
}

func extensionQuestionIDs(questions []briefdomain.ArticleQuestion) []string {
	base := map[string]bool{}
	for _, id := range baseQuestionIDs() {
		base[id] = true
	}
	ids := make([]string, 0)
	for _, question := range questions {
		if question.FlowType == briefdomain.QuestionFlowMain && !base[question.ID] {
			ids = append(ids, question.ID)
		}
	}
	return ids
}

func questionTemplate(questions []briefdomain.ArticleQuestion) []questionEntry {
	entries := make([]questionEntry, 0, len(questions))
	for _, question := range questions {
		if question.FlowType != briefdomain.QuestionFlowMain {
			continue
		}
		entries = append(entries, questionEntry{
			ID:          question.ID,
			Text:        question.Text,
			Required:    question.Required,
			TargetField: question.TargetField,
		})
	}
	return entries
}

func customAnswerIDs(answers []briefdomain.BriefAnswer) []string {
	ids := make([]string, 0, len(answers))
	for _, answer := range answers {
		ids = append(ids, answer.QuestionID)
	}
	return ids
}

func deepDiveTargetIDs(answers []briefdomain.BriefAnswer) []string {
	ids := make([]string, 0, len(answers))
	for _, answer := range answers {
		ids = append(ids, answer.TargetQuestionID)
	}
	return ids
}

func questionCoverage(results []caseResult) map[string]int {
	coverage := map[string]int{"cases": len(results)}
	for _, result := range results {
		for _, question := range result.QuestionTemplate {
			coverage[question.ID]++
		}
	}
	return coverage
}

func briefCoverage(results []caseResult) map[string]int {
	coverage := map[string]int{"cases": len(results)}
	for _, result := range results {
		if allChecksPassed(result.BriefChecks) {
			coverage["passing_briefs"]++
		}
		if result.DeepDiveCount == briefdomain.MaxTotalFollowUps {
			coverage["max_deep_dive_briefs"]++
		}
	}
	return coverage
}

func casesMarkdown(report scenarioReport) string {
	var builder strings.Builder
	builder.WriteString("# Interview template scenario\n\n")
	builder.WriteString("- Generated by: `cmd/scenario/interview_template`\n")
	builder.WriteString(fmt.Sprintf("- Offline only: `%v`\n", report.OfflineOnly))
	builder.WriteString(fmt.Sprintf("- Cases: `%d`\n\n", len(report.Cases)))
	builder.WriteString("| Case | Persona | Format | Questions | Custom answers | Deep dives | Checks | Brief |\n")
	builder.WriteString("|---|---|---|---:|---:|---:|---|---|\n")
	for _, result := range report.Cases {
		builder.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | %d | %d | %d | %s | `%s` |\n",
			result.ID,
			result.PersonaID,
			result.OutputFormatID,
			result.QuestionCount,
			len(result.CustomAnswerIDs),
			result.DeepDiveCount,
			checkStatus(result),
			result.BriefPath,
		))
	}
	return builder.String()
}

func checkStatus(result caseResult) string {
	if allChecksPassed(result.TemplateChecks) && allChecksPassed(result.BriefChecks) {
		return "passed"
	}
	return "failed"
}

func allChecksPassed(checks []checkResult) bool {
	for _, item := range checks {
		if !item.Passed {
			return false
		}
	}
	return true
}

func check(name string, passed bool, detail string) checkResult {
	return checkResult{Name: name, Passed: passed, Detail: detail}
}

func uniqueQuestionIDs(questions []briefdomain.ArticleQuestion) bool {
	seen := map[string]bool{}
	for _, question := range questions {
		if seen[question.ID] {
			return false
		}
		seen[question.ID] = true
	}
	return true
}

func containsAllQuestionIDs(questions []briefdomain.ArticleQuestion, ids []string) bool {
	for _, id := range ids {
		if !containsQuestionID(questions, id) {
			return false
		}
	}
	return true
}

func containsQuestionID(questions []briefdomain.ArticleQuestion, id string) bool {
	for _, question := range questions {
		if question.ID == id {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func writeJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return writeFile(path, string(encoded)+"\n")
}

func writeFile(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
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
