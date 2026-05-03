package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMatrixDir   = "tmp/media_matrix"
	defaultLiveDir     = "tmp/media_matrix/live"
	offlineGeneratedAt = "1970-01-01T00:00:00Z"
)

type matrixOutput struct {
	Cases []matrixCase `json:"cases"`
}

type matrixCase struct {
	ID                    string        `json:"id"`
	PersonaID             string        `json:"persona_id"`
	OutputFormatID        string        `json:"output_format_id"`
	Medium                string        `json:"medium"`
	Style                 string        `json:"style"`
	Theme                 string        `json:"theme"`
	TargetLengthStructure string        `json:"target_length_structure"`
	SourceSelectors       []string      `json:"source_selectors"`
	BriefPath             string        `json:"brief_path"`
	ProfilePath           string        `json:"profile_path"`
	GuidePath             string        `json:"guide_path"`
	PromptPath            string        `json:"prompt_path"`
	ActiveGates           scenarioGates `json:"active_gates"`
}

type aggregateReport struct {
	GeneratedBy     string           `json:"generated_by"`
	Live            bool             `json:"live"`
	MatrixPath      string           `json:"matrix_path"`
	GeneratedAt     string           `json:"generated_at"`
	SelectedCaseIDs []string         `json:"selected_case_ids"`
	RunOrdinals     []int            `json:"run_ordinals"`
	RepeatRuns      int              `json:"repeat_runs"`
	Runtime         runtimeMetadata  `json:"runtime"`
	Summary         aggregateSummary `json:"summary"`
	Rows            []resultRow      `json:"rows"`
}

type runtimeMetadata struct {
	LLMBaseURLs  []string `json:"llm_base_urls,omitempty"`
	LLMModels    []string `json:"llm_models,omitempty"`
	VerifyModels []string `json:"verify_models,omitempty"`
	TailnetEvoX2 bool     `json:"tailnet_evo_x2"`
}

type aggregateSummary struct {
	TotalRows      int                   `json:"total_rows"`
	SelectedCases  int                   `json:"selected_cases"`
	PassedRows     int                   `json:"passed_rows"`
	FailedRows     int                   `json:"failed_rows"`
	PlannedRows    int                   `json:"planned_rows"`
	SkippedRows    int                   `json:"skipped_rows"`
	OtherRows      int                   `json:"other_rows"`
	ConciseRows    []conciseAggregateRow `json:"concise_rows"`
	PassFailGroups []outcomeGroup        `json:"pass_fail_groups"`
}

type conciseAggregateRow struct {
	RunOrdinal     int      `json:"run_ordinal"`
	SelectedCases  int      `json:"selected_cases"`
	PassedRows     int      `json:"passed_rows"`
	FailedRows     int      `json:"failed_rows"`
	PlannedRows    int      `json:"planned_rows"`
	SkippedRows    int      `json:"skipped_rows"`
	OtherRows      int      `json:"other_rows"`
	AverageSeconds float64  `json:"average_seconds,omitempty"`
	AverageScore   float64  `json:"average_score,omitempty"`
	AverageRunes   int      `json:"average_runes,omitempty"`
	LLMBaseURLs    []string `json:"llm_base_urls,omitempty"`
	LLMModels      []string `json:"llm_models,omitempty"`
	VerifyModels   []string `json:"verify_models,omitempty"`
}

type outcomeGroup struct {
	Outcome    string   `json:"outcome"`
	Count      int      `json:"count"`
	CaseRunIDs []string `json:"case_run_ids"`
}

type resultRow struct {
	RunOrdinal            int           `json:"run_ordinal"`
	ComparisonKey         string        `json:"comparison_key"`
	CaseID                string        `json:"case_id"`
	Medium                string        `json:"medium"`
	Style                 string        `json:"style"`
	PersonaID             string        `json:"persona_id"`
	OutputFormatID        string        `json:"output_format_id"`
	Theme                 string        `json:"theme"`
	TargetLengthStructure string        `json:"target_length_structure"`
	SourceSelectors       []string      `json:"source_selectors"`
	Status                string        `json:"status"`
	ActiveGates           scenarioGates `json:"active_gates"`
	FailureGroup          string        `json:"failure_group,omitempty"`
	ElapsedSeconds        float64       `json:"elapsed_seconds,omitempty"`
	FirstChunkMS          int           `json:"first_chunk_ms,omitempty"`
	Chunks                int           `json:"chunks,omitempty"`
	Score                 float64       `json:"score,omitempty"`
	MinStyleScore         float64       `json:"min_style_score,omitempty"`
	Runes                 int           `json:"runes,omitempty"`
	MinRunes              int           `json:"min_runes,omitempty"`
	Passed                bool          `json:"passed,omitempty"`
	ScenarioPassed        bool          `json:"scenario_passed,omitempty"`
	VerificationPerformed bool          `json:"verification_performed,omitempty"`
	VerificationPassed    bool          `json:"verification_passed,omitempty"`
	LLMBaseURL            string        `json:"llm_base_url,omitempty"`
	LLMModel              string        `json:"llm_model,omitempty"`
	VerifyModel           string        `json:"verify_model,omitempty"`
	OutputDir             string        `json:"output_dir"`
	ProfilePath           string        `json:"profile_path,omitempty"`
	GuidePath             string        `json:"guide_path,omitempty"`
	DraftPath             string        `json:"draft_path,omitempty"`
	EvaluationPath        string        `json:"evaluation_path,omitempty"`
	VerificationPath      string        `json:"verification_path,omitempty"`
	FailurePath           string        `json:"failure_path,omitempty"`
	RawOutputPaths        []string      `json:"raw_output_paths,omitempty"`
	Error                 string        `json:"error,omitempty"`
}

type scenarioGates struct {
	MinRunes             int      `json:"min_runes"`
	MinStyleScore        float64  `json:"min_style_score"`
	StructuralGateLabels []string `json:"structural_gate_labels"`
	StructuralSignals    []string `json:"structural_signals"`
}

func main() {
	matrixDir := envOrDefault("SCENARIO_OUTPUT_DIR", defaultMatrixDir)
	matrixPath := filepath.Join(matrixDir, "matrix.json")
	liveDir := envOrDefault("LIVE_MEDIA_MATRIX_OUTPUT_DIR", defaultLiveDir)
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		fatalf("create live output dir: %v", err)
	}

	matrix := readMatrix(matrixPath)
	live := os.Getenv("RUN_LIVE_MEDIA_MATRIX") == "1"
	selected := selectedCaseIDs(os.Getenv("LIVE_MEDIA_MATRIX_CASES"))
	cases := selectedCases(matrix.Cases, selected)
	startOrdinal := positiveEnvInt("LIVE_MEDIA_MATRIX_RUN_ORDINAL", 1)
	repeatRuns := positiveEnvInt("LIVE_MEDIA_MATRIX_REPEAT_RUNS", 1)
	runOrdinals := runOrdinalRange(startOrdinal, repeatRuns)
	rows := make([]resultRow, 0, len(cases)*repeatRuns)
	for _, ordinal := range runOrdinals {
		for _, item := range cases {
			outputDir := outputDirForRun(liveDir, item.ID, ordinal, repeatRuns)
			if !live {
				rows = append(rows, plannedRowForRun(item, outputDir, ordinal))
				continue
			}
			rows = append(rows, runCaseForRun(item, outputDir, ordinal))
		}
	}
	if len(rows) == 0 {
		fatalf("no media matrix cases selected")
	}

	selectedIDs := caseIDs(cases)
	report := aggregateReport{
		GeneratedBy:     "cmd/scenario/live_media_matrix",
		Live:            live,
		MatrixPath:      matrixPath,
		GeneratedAt:     generatedAt(live),
		SelectedCaseIDs: selectedIDs,
		RunOrdinals:     runOrdinals,
		RepeatRuns:      repeatRuns,
		Runtime:         runtimeFromRows(rows),
		Summary:         summarizeRows(rows, len(selectedIDs)),
		Rows:            rows,
	}
	writeJSON(filepath.Join(liveDir, "aggregate.json"), report)
	writeFile(filepath.Join(liveDir, "aggregate.md"), markdownReport(report))

	fmt.Printf("live media matrix completed\n")
	fmt.Printf("live=%v\n", live)
	fmt.Printf("cases=%d\n", len(rows))
	fmt.Printf("aggregate=%s\n", filepath.Join(liveDir, "aggregate.json"))
	fmt.Printf("report=%s\n", filepath.Join(liveDir, "aggregate.md"))
	if live && hasFailure(rows) {
		os.Exit(1)
	}
}

func readMatrix(path string) matrixOutput {
	encoded, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	var matrix matrixOutput
	if err := json.Unmarshal(encoded, &matrix); err != nil {
		fatalf("decode %s: %v", path, err)
	}
	if len(matrix.Cases) == 0 {
		fatalf("%s has no cases", path)
	}
	return matrix
}

func plannedRow(item matrixCase, outputDir string) resultRow {
	return plannedRowForRun(item, outputDir, 1)
}

func plannedRowForRun(item matrixCase, outputDir string, runOrdinal int) resultRow {
	return resultRow{
		RunOrdinal:            runOrdinal,
		ComparisonKey:         comparisonKey(runOrdinal, item.ID),
		CaseID:                item.ID,
		Medium:                item.Medium,
		Style:                 item.Style,
		PersonaID:             item.PersonaID,
		OutputFormatID:        item.OutputFormatID,
		Theme:                 item.Theme,
		TargetLengthStructure: item.TargetLengthStructure,
		SourceSelectors:       append([]string(nil), item.SourceSelectors...),
		Status:                "planned",
		ActiveGates:           item.ActiveGates,
		MinStyleScore:         item.ActiveGates.MinStyleScore,
		MinRunes:              item.ActiveGates.MinRunes,
		OutputDir:             outputDir,
		ProfilePath:           item.ProfilePath,
		GuidePath:             item.GuidePath,
	}
}

func runCase(item matrixCase, outputDir string) resultRow {
	return runCaseForRun(item, outputDir, 1)
}

func runCaseForRun(item matrixCase, outputDir string, runOrdinal int) resultRow {
	row := plannedRowForRun(item, outputDir, runOrdinal)
	row.Status = "running"
	applyRuntimeEnv(&row)
	if envBool("LIVE_MEDIA_MATRIX_RESUME") && fileExists(filepath.Join(outputDir, "draft.md")) {
		row.Status = "skipped_existing"
		row.DraftPath = filepath.Join(outputDir, "draft.md")
		row.EvaluationPath = filepath.Join(outputDir, "evaluation.json")
		row.VerificationPath = filepath.Join(outputDir, "verification.json")
		applyRunMetrics(&row, readKeyValuesFile(filepath.Join(outputDir, "stdout.txt")), item.ActiveGates)
		applyStructuralGates(&row)
		return row
	}
	if err := os.RemoveAll(outputDir); err != nil {
		row.Status = "failed"
		row.Error = err.Error()
		return row
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		row.Status = "failed"
		row.Error = err.Error()
		return row
	}

	cmd := exec.Command("go", "run", "./cmd/scenario/draft_generation")
	cmd.Env = draftGenerationEnv(item, outputDir)
	if os.Getenv("SCENARIO_STREAM_DRAFT") == "" {
		cmd.Env = append(cmd.Env, "SCENARIO_STREAM_DRAFT=1")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	writeFile(filepath.Join(outputDir, "stdout.txt"), stdout.String())
	writeFile(filepath.Join(outputDir, "stderr.txt"), stderr.String())

	applyRunMetrics(&row, parseKeyValues(stdout.String()), item.ActiveGates)
	applyStructuralGates(&row)
	if err != nil {
		row.Status = "failed"
		row.Error = strings.TrimSpace(stderr.String())
		if row.Error == "" {
			row.Error = err.Error()
		}
		applyFailureArtifacts(&row, outputDir)
		if row.FailureGroup == "" {
			row.FailureGroup = failureGroup(row)
		}
		return row
	}
	if row.Status == "running" {
		row.Status = "passed"
	}
	if rowOutcome(row) != "passed" {
		row.Status = "failed"
		if row.FailureGroup == "" {
			row.FailureGroup = failureGroup(row)
		}
		return row
	}
	return row
}

func selectedCases(cases []matrixCase, selected map[string]bool) []matrixCase {
	if len(selected) == 0 {
		return append([]matrixCase(nil), cases...)
	}
	filtered := make([]matrixCase, 0, len(selected))
	for _, item := range cases {
		if selected[item.ID] {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func caseIDs(cases []matrixCase) []string {
	ids := make([]string, 0, len(cases))
	for _, item := range cases {
		ids = append(ids, item.ID)
	}
	return ids
}

func outputDirForRun(liveDir, caseID string, runOrdinal, repeatRuns int) string {
	if repeatRuns == 1 && runOrdinal == 1 {
		return filepath.Join(liveDir, caseID)
	}
	return filepath.Join(liveDir, fmt.Sprintf("run_%02d", runOrdinal), caseID)
}

func runOrdinalRange(startOrdinal, repeatRuns int) []int {
	ordinals := make([]int, 0, repeatRuns)
	for offset := 0; offset < repeatRuns; offset++ {
		ordinals = append(ordinals, startOrdinal+offset)
	}
	return ordinals
}

func comparisonKey(runOrdinal int, caseID string) string {
	return fmt.Sprintf("run_%02d/%s", runOrdinal, caseID)
}

func positiveEnvInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		fatalf("%s must be a positive integer", name)
	}
	return parsed
}

func generatedAt(live bool) string {
	if value := strings.TrimSpace(os.Getenv("LIVE_MEDIA_MATRIX_GENERATED_AT")); value != "" {
		return value
	}
	if !live {
		return offlineGeneratedAt
	}
	return time.Now().UTC().Format(time.RFC3339)
}

func applyRuntimeEnv(row *resultRow) {
	row.LLMBaseURL = valueOrDefault(row.LLMBaseURL, os.Getenv("LLM_BASE_URL"))
	row.LLMModel = valueOrDefault(row.LLMModel, valueOrDefault(os.Getenv("DRAFT_LLM_MODEL"), os.Getenv("LLM_MODEL")))
	row.VerifyModel = valueOrDefault(row.VerifyModel, os.Getenv("VERIFY_LLM_MODEL"))
}

func applyRunMetrics(row *resultRow, values map[string]string, gates scenarioGates) {
	if len(values) == 0 {
		return
	}
	row.ElapsedSeconds = floatValue(values["elapsed_seconds"])
	row.FirstChunkMS = intValue(values["first_chunk_ms"])
	row.Chunks = intValue(values["chunks"])
	row.Score = floatValue(values["score"])
	row.MinStyleScore = floatValueOrDefault(values["min_style_score"], gates.MinStyleScore)
	row.Runes = intValue(values["runes"])
	row.MinRunes = intValueOrDefault(values["min_draft_runes"], gates.MinRunes)
	row.Passed = boolValue(values["passed"])
	row.ScenarioPassed = boolValue(values["scenario_passed"])
	row.VerificationPerformed = boolValue(values["verification_performed"])
	row.VerificationPassed = boolValue(values["verification_passed"])
	row.LLMBaseURL = valueOrDefault(values["llm_base_url"], row.LLMBaseURL)
	row.LLMModel = valueOrDefault(values["llm_model"], row.LLMModel)
	row.VerifyModel = valueOrDefault(values["verify_model"], row.VerifyModel)
	row.DraftPath = valueOrDefault(values["draft"], row.DraftPath)
	row.EvaluationPath = valueOrDefault(values["evaluation"], row.EvaluationPath)
	row.VerificationPath = valueOrDefault(values["verification"], row.VerificationPath)
	row.FailureGroup = failureGroup(*row)
}

func applyStructuralGates(row *resultRow) {
	if len(row.ActiveGates.StructuralSignals) == 0 || strings.TrimSpace(row.DraftPath) == "" {
		return
	}
	content, err := os.ReadFile(row.DraftPath)
	if err != nil {
		return
	}
	missing := missingStructuralSignals(string(content), row.ActiveGates.StructuralSignals)
	if len(missing) == 0 {
		return
	}
	row.Status = "failed"
	row.ScenarioPassed = false
	row.FailureGroup = "structural_gate"
	detail := "missing structural signals: " + strings.Join(missing, ", ")
	if strings.TrimSpace(row.Error) == "" {
		row.Error = detail
	} else if !strings.Contains(row.Error, detail) {
		row.Error += "\n" + detail
	}
}

func missingStructuralSignals(content string, signals []string) []string {
	missing := make([]string, 0)
	for _, signal := range signals {
		if strings.TrimSpace(signal) == "" {
			continue
		}
		if !strings.Contains(content, signal) {
			missing = append(missing, signal)
		}
	}
	return missing
}

func draftGenerationEnv(item matrixCase, outputDir string) []string {
	env := append(os.Environ(),
		"RUN_LOCAL_LLM_SCENARIO=1",
		"ARTICLE_BRIEF_PATH="+item.BriefPath,
		"SCENARIO_OUTPUT_DIR="+outputDir,
	)
	if strings.TrimSpace(item.ProfilePath) != "" {
		env = append(env, "AUTHOR_PROFILE_PATH="+item.ProfilePath)
	}
	if strings.TrimSpace(item.GuidePath) != "" {
		env = append(env, "WRITING_GUIDE_PATH="+item.GuidePath)
	}
	if item.ActiveGates.MinStyleScore > 0 {
		env = append(env, fmt.Sprintf("SCENARIO_MIN_STYLE_SCORE=%.1f", item.ActiveGates.MinStyleScore))
	}
	if item.ActiveGates.MinRunes > 0 {
		env = append(env, fmt.Sprintf("SCENARIO_MIN_DRAFT_RUNES=%d", item.ActiveGates.MinRunes))
	}
	return env
}

func failureGroup(row resultRow) string {
	switch {
	case strings.TrimSpace(row.FailurePath) != "":
		return "generation_or_validation"
	case row.VerificationPerformed && !row.VerificationPassed:
		return "final_verification"
	case row.Score > 0 && row.MinStyleScore > 0 && row.Score < row.MinStyleScore:
		return "style_score"
	case row.Runes > 0 && row.MinRunes > 0 && row.Runes < row.MinRunes:
		return "draft_length"
	case strings.TrimSpace(row.Error) != "":
		return "runtime_or_runner"
	default:
		return ""
	}
}

type failureAttemptReport struct {
	RuntimeMetrics attemptRuntimeMetrics `json:"runtime_metrics"`
	Context        failureContext        `json:"context"`
	RawOutputs     []rawAttemptArtifact  `json:"raw_outputs"`
}

type attemptRuntimeMetrics struct {
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	FirstChunkMs   int     `json:"first_chunk_ms,omitempty"`
	Chunks         int     `json:"chunks,omitempty"`
}

type failureContext struct {
	LLMBaseURL     string `json:"llm_base_url"`
	LLMModel       string `json:"llm_model"`
	VerifyModel    string `json:"verify_model"`
	OutputFormatID string `json:"output_format_id"`
}

type rawAttemptArtifact struct {
	Path string `json:"path"`
}

func applyFailureArtifacts(row *resultRow, outputDir string) {
	path := latestFailureAttemptPath(outputDir)
	if path == "" {
		return
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var report failureAttemptReport
	if err := json.Unmarshal(encoded, &report); err != nil {
		return
	}
	row.FailurePath = path
	if row.ElapsedSeconds == 0 {
		row.ElapsedSeconds = report.RuntimeMetrics.ElapsedSeconds
	}
	if row.FirstChunkMS == 0 {
		row.FirstChunkMS = report.RuntimeMetrics.FirstChunkMs
	}
	if row.Chunks == 0 {
		row.Chunks = report.RuntimeMetrics.Chunks
	}
	row.LLMBaseURL = valueOrDefault(row.LLMBaseURL, report.Context.LLMBaseURL)
	row.LLMModel = valueOrDefault(row.LLMModel, report.Context.LLMModel)
	row.VerifyModel = valueOrDefault(row.VerifyModel, report.Context.VerifyModel)
	for _, raw := range report.RawOutputs {
		if strings.TrimSpace(raw.Path) != "" {
			row.RawOutputPaths = append(row.RawOutputPaths, raw.Path)
		}
	}
	row.FailureGroup = failureGroup(*row)
}

func latestFailureAttemptPath(outputDir string) string {
	matches, err := filepath.Glob(filepath.Join(outputDir, "failure_attempt_*.json"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[len(matches)-1]
}

func parseKeyValues(output string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return values
}

func readKeyValuesFile(path string) map[string]string {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseKeyValues(string(content))
}

func markdownReport(report aggregateReport) string {
	summary := report.Summary
	if summary.TotalRows == 0 {
		summary = summarizeRows(report.Rows, len(report.SelectedCaseIDs))
	}
	var builder strings.Builder
	builder.WriteString("# Live media matrix aggregate\n\n")
	builder.WriteString(fmt.Sprintf("- Generated at: `%s`\n", report.GeneratedAt))
	builder.WriteString(fmt.Sprintf("- Live mode: `%v`\n", report.Live))
	builder.WriteString(fmt.Sprintf("- Matrix: `%s`\n", report.MatrixPath))
	builder.WriteString(fmt.Sprintf("- Selected cases: `%s`\n", strings.Join(report.SelectedCaseIDs, ", ")))
	builder.WriteString(fmt.Sprintf("- Run ordinals: `%s`\n", joinInts(report.RunOrdinals)))
	builder.WriteString(fmt.Sprintf("- Tailnet Evo X2: `%v`\n\n", report.Runtime.TailnetEvoX2))

	builder.WriteString("## Summary\n\n")
	builder.WriteString("| Run | Cases | Passed | Failed | Planned | Skipped | Seconds | Score | Runes | Runtime |\n")
	builder.WriteString("|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for _, row := range summary.ConciseRows {
		builder.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d | %d | %.2f | %.1f | %d | %s |\n",
			row.RunOrdinal,
			row.SelectedCases,
			row.PassedRows,
			row.FailedRows,
			row.PlannedRows,
			row.SkippedRows,
			row.AverageSeconds,
			row.AverageScore,
			row.AverageRunes,
			escapePipes(runtimeLabel(row.LLMBaseURLs, row.LLMModels)),
		))
	}

	for _, group := range summary.PassFailGroups {
		if group.Count == 0 {
			continue
		}
		builder.WriteString(fmt.Sprintf("\n## %s\n\n", titleOutcome(group.Outcome)))
		builder.WriteString(fmt.Sprintf("Cases: `%s`\n", strings.Join(group.CaseRunIDs, ", ")))
		builder.WriteString("\n")
	}

	builder.WriteString("## Case Results\n\n")
	builder.WriteString("| Run | Case | Medium | Style | Outcome | Status | Gates | Seconds | Score | Runes | Verification | Output |\n")
	builder.WriteString("|---:|---|---|---|---|---|---|---:|---:|---:|---|---|\n")
	for _, row := range report.Rows {
		builder.WriteString(fmt.Sprintf("| %d | `%s` | %s | %s | %s | %s | %s | %.2f | %.1f / %.1f | %d / %d | %v | `%s` |\n",
			row.RunOrdinal,
			row.CaseID,
			escapePipes(row.Medium),
			escapePipes(row.Style),
			rowOutcome(row),
			row.Status,
			escapePipes(gateSummary(row.ActiveGates)),
			row.ElapsedSeconds,
			row.Score,
			row.MinStyleScore,
			row.Runes,
			row.MinRunes,
			row.VerificationPassed,
			row.OutputDir,
		))
	}
	failures := failedRows(report.Rows)
	if len(failures) > 0 {
		builder.WriteString("\n## Failure Details\n\n")
		for _, row := range failures {
			builder.WriteString(fmt.Sprintf("- Run %d `%s`: %s\n", row.RunOrdinal, row.CaseID, failureReason(row)))
		}
	}
	return builder.String()
}

func summarizeRows(rows []resultRow, selectedCases int) aggregateSummary {
	if selectedCases == 0 {
		selectedCases = len(uniqueCaseIDs(rows))
	}
	summary := aggregateSummary{
		TotalRows:     len(rows),
		SelectedCases: selectedCases,
	}
	byRun := map[int][]resultRow{}
	groups := map[string][]string{}
	for _, row := range rows {
		outcome := rowOutcome(row)
		switch outcome {
		case "passed":
			summary.PassedRows++
		case "failed":
			summary.FailedRows++
		case "planned":
			summary.PlannedRows++
		case "skipped_existing":
			summary.SkippedRows++
		default:
			summary.OtherRows++
		}
		byRun[row.RunOrdinal] = append(byRun[row.RunOrdinal], row)
		groups[outcome] = append(groups[outcome], row.ComparisonKey)
	}
	ordinals := make([]int, 0, len(byRun))
	for ordinal := range byRun {
		ordinals = append(ordinals, ordinal)
	}
	sort.Ints(ordinals)
	for _, ordinal := range ordinals {
		summary.ConciseRows = append(summary.ConciseRows, summarizeRunRows(ordinal, byRun[ordinal], selectedCases))
	}
	for _, outcome := range []string{"failed", "passed", "planned", "skipped_existing", "other"} {
		caseIDs := sortedUnique(groups[outcome])
		summary.PassFailGroups = append(summary.PassFailGroups, outcomeGroup{
			Outcome:    outcome,
			Count:      len(caseIDs),
			CaseRunIDs: caseIDs,
		})
	}
	return summary
}

func summarizeRunRows(runOrdinal int, rows []resultRow, selectedCases int) conciseAggregateRow {
	row := conciseAggregateRow{
		RunOrdinal:    runOrdinal,
		SelectedCases: selectedCases,
	}
	var secondsTotal float64
	var secondsCount int
	var scoreTotal float64
	var scoreCount int
	var runeTotal int
	var runeCount int
	for _, item := range rows {
		switch rowOutcome(item) {
		case "passed":
			row.PassedRows++
		case "failed":
			row.FailedRows++
		case "planned":
			row.PlannedRows++
		case "skipped_existing":
			row.SkippedRows++
		default:
			row.OtherRows++
		}
		if item.ElapsedSeconds > 0 {
			secondsTotal += item.ElapsedSeconds
			secondsCount++
		}
		if item.Score > 0 {
			scoreTotal += item.Score
			scoreCount++
		}
		if item.Runes > 0 {
			runeTotal += item.Runes
			runeCount++
		}
		row.LLMBaseURLs = appendIfPresent(row.LLMBaseURLs, item.LLMBaseURL)
		row.LLMModels = appendIfPresent(row.LLMModels, item.LLMModel)
		row.VerifyModels = appendIfPresent(row.VerifyModels, item.VerifyModel)
	}
	row.LLMBaseURLs = sortedUnique(row.LLMBaseURLs)
	row.LLMModels = sortedUnique(row.LLMModels)
	row.VerifyModels = sortedUnique(row.VerifyModels)
	row.AverageSeconds = averageFloat(secondsTotal, secondsCount)
	row.AverageScore = averageFloat(scoreTotal, scoreCount)
	row.AverageRunes = averageInt(runeTotal, runeCount)
	return row
}

func runtimeFromRows(rows []resultRow) runtimeMetadata {
	var runtime runtimeMetadata
	for _, row := range rows {
		runtime.LLMBaseURLs = appendIfPresent(runtime.LLMBaseURLs, row.LLMBaseURL)
		runtime.LLMModels = appendIfPresent(runtime.LLMModels, row.LLMModel)
		runtime.VerifyModels = appendIfPresent(runtime.VerifyModels, row.VerifyModel)
	}
	runtime.LLMBaseURLs = sortedUnique(runtime.LLMBaseURLs)
	runtime.LLMModels = sortedUnique(runtime.LLMModels)
	runtime.VerifyModels = sortedUnique(runtime.VerifyModels)
	for _, baseURL := range runtime.LLMBaseURLs {
		if isTailnetEvoX2(baseURL) {
			runtime.TailnetEvoX2 = true
			break
		}
	}
	return runtime
}

func isTailnetEvoX2(baseURL string) bool {
	cleaned := strings.ToLower(strings.TrimSpace(baseURL))
	return strings.Contains(cleaned, "evo-x2") && !strings.Contains(cleaned, "127.0.0.1") && !strings.Contains(cleaned, "localhost")
}

func rowOutcome(row resultRow) string {
	switch row.Status {
	case "planned":
		return "planned"
	case "failed":
		return "failed"
	case "passed":
		if row.ScenarioPassed {
			return "passed"
		}
		return "failed"
	case "skipped_existing":
		if row.ScenarioPassed || row.Passed {
			return "passed"
		}
		return "skipped_existing"
	default:
		if strings.TrimSpace(row.Status) == "" {
			return "other"
		}
		return row.Status
	}
}

func failureReason(row resultRow) string {
	if strings.TrimSpace(row.Error) != "" {
		return strings.TrimSpace(row.Error)
	}
	if row.Status == "passed" && !row.ScenarioPassed {
		return "scenario_passed=false"
	}
	return row.Status
}

func titleOutcome(outcome string) string {
	cleaned := strings.ReplaceAll(strings.TrimSpace(outcome), "_", " ")
	if cleaned == "" {
		return "Other"
	}
	return strings.ToUpper(cleaned[:1]) + cleaned[1:]
}

func gateSummary(gates scenarioGates) string {
	if gates.MinRunes == 0 && gates.MinStyleScore == 0 && len(gates.StructuralGateLabels) == 0 {
		return ""
	}
	return fmt.Sprintf(
		"min %.1f style / %d runes; %s",
		gates.MinStyleScore,
		gates.MinRunes,
		strings.Join(gates.StructuralGateLabels, ", "),
	)
}

func failedRows(rows []resultRow) []resultRow {
	failures := make([]resultRow, 0)
	for _, row := range rows {
		if rowOutcome(row) == "failed" {
			failures = append(failures, row)
		}
	}
	return failures
}

func hasFailure(rows []resultRow) bool {
	return len(failedRows(rows)) > 0
}

func selectedCaseIDs(value string) map[string]bool {
	parts := splitCSV(value)
	if len(parts) == 0 {
		return nil
	}
	selected := make(map[string]bool, len(parts))
	for _, part := range parts {
		selected[part] = true
	}
	return selected
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if cleaned := strings.TrimSpace(part); cleaned != "" {
			values = append(values, cleaned)
		}
	}
	sort.Strings(values)
	return values
}

func joinInts(values []int) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ", ")
}

func uniqueCaseIDs(rows []resultRow) []string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		values = appendIfPresent(values, row.CaseID)
	}
	return sortedUnique(values)
}

func sortedUnique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned == "" || seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		unique = append(unique, cleaned)
	}
	sort.Strings(unique)
	return unique
}

func appendIfPresent(values []string, value string) []string {
	if strings.TrimSpace(value) == "" {
		return values
	}
	return append(values, strings.TrimSpace(value))
}

func averageFloat(total float64, count int) float64 {
	if count == 0 {
		return 0
	}
	return math.Round(total/float64(count)*100) / 100
}

func averageInt(total int, count int) int {
	if count == 0 {
		return 0
	}
	return int(math.Round(float64(total) / float64(count)))
}

func runtimeLabel(baseURLs, models []string) string {
	baseURL := "n/a"
	model := "n/a"
	if len(baseURLs) > 0 {
		baseURL = strings.Join(baseURLs, ", ")
	}
	if len(models) > 0 {
		model = strings.Join(models, ", ")
	}
	return fmt.Sprintf("%s / %s", baseURL, model)
}

func boolValue(value string) bool {
	parsed, _ := strconv.ParseBool(value)
	return parsed
}

func intValue(value string) int {
	parsed, _ := strconv.Atoi(value)
	return parsed
}

func intValueOrDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func floatValue(value string) float64 {
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func floatValueOrDefault(value string, fallback float64) float64 {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string) bool {
	return os.Getenv(key) == "1" || strings.EqualFold(os.Getenv(key), "true")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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

func escapePipes(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
