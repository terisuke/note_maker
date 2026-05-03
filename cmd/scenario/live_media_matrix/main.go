package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMatrixDir = "tmp/media_matrix"
	defaultLiveDir   = "tmp/media_matrix/live"
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
	PromptPath            string        `json:"prompt_path"`
	ActiveGates           scenarioGates `json:"active_gates"`
}

type aggregateReport struct {
	GeneratedBy string      `json:"generated_by"`
	Live        bool        `json:"live"`
	MatrixPath  string      `json:"matrix_path"`
	GeneratedAt string      `json:"generated_at"`
	Rows        []resultRow `json:"rows"`
}

type resultRow struct {
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
	rows := make([]resultRow, 0, len(matrix.Cases))
	for _, item := range matrix.Cases {
		if len(selected) > 0 && !selected[item.ID] {
			continue
		}
		if !live {
			rows = append(rows, plannedRow(item, filepath.Join(liveDir, item.ID)))
			continue
		}
		rows = append(rows, runCase(item, filepath.Join(liveDir, item.ID)))
	}
	if len(rows) == 0 {
		fatalf("no media matrix cases selected")
	}

	report := aggregateReport{
		GeneratedBy: "cmd/scenario/live_media_matrix",
		Live:        live,
		MatrixPath:  matrixPath,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Rows:        rows,
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
	return resultRow{
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
	}
}

func runCase(item matrixCase, outputDir string) resultRow {
	row := plannedRow(item, outputDir)
	row.Status = "running"
	if envBool("LIVE_MEDIA_MATRIX_RESUME") && fileExists(filepath.Join(outputDir, "draft.md")) {
		row.Status = "skipped_existing"
		row.DraftPath = filepath.Join(outputDir, "draft.md")
		row.EvaluationPath = filepath.Join(outputDir, "evaluation.json")
		row.VerificationPath = filepath.Join(outputDir, "verification.json")
		applyRunMetrics(&row, readKeyValuesFile(filepath.Join(outputDir, "stdout.txt")), item.ActiveGates)
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
	if err != nil {
		row.Status = "failed"
		row.Error = strings.TrimSpace(stderr.String())
		if row.Error == "" {
			row.Error = err.Error()
		}
		applyFailureArtifacts(&row, outputDir)
		return row
	}
	row.Status = "passed"
	return row
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
	row.LLMBaseURL = values["llm_base_url"]
	row.LLMModel = values["llm_model"]
	row.VerifyModel = values["verify_model"]
	row.DraftPath = valueOrDefault(values["draft"], row.DraftPath)
	row.EvaluationPath = valueOrDefault(values["evaluation"], row.EvaluationPath)
	row.VerificationPath = valueOrDefault(values["verification"], row.VerificationPath)
}

func draftGenerationEnv(item matrixCase, outputDir string) []string {
	env := append(os.Environ(),
		"RUN_LOCAL_LLM_SCENARIO=1",
		"ARTICLE_BRIEF_PATH="+item.BriefPath,
		"SCENARIO_OUTPUT_DIR="+outputDir,
	)
	if item.ActiveGates.MinStyleScore > 0 {
		env = append(env, fmt.Sprintf("SCENARIO_MIN_STYLE_SCORE=%.1f", item.ActiveGates.MinStyleScore))
	}
	if item.ActiveGates.MinRunes > 0 {
		env = append(env, fmt.Sprintf("SCENARIO_MIN_DRAFT_RUNES=%d", item.ActiveGates.MinRunes))
	}
	return env
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
	var builder strings.Builder
	builder.WriteString("# Live media matrix aggregate\n\n")
	builder.WriteString(fmt.Sprintf("- Generated at: `%s`\n", report.GeneratedAt))
	builder.WriteString(fmt.Sprintf("- Live mode: `%v`\n", report.Live))
	builder.WriteString(fmt.Sprintf("- Matrix: `%s`\n\n", report.MatrixPath))
	builder.WriteString("| Case | Medium | Style | Status | Gates | Seconds | Score | Runes | Verification | Output |\n")
	builder.WriteString("|---|---|---|---|---|---:|---:|---:|---|---|\n")
	for _, row := range report.Rows {
		builder.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s | %.2f | %.1f / %.1f | %d / %d | %v | `%s` |\n",
			row.CaseID,
			escapePipes(row.Medium),
			escapePipes(row.Style),
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
		builder.WriteString("\n## Failures\n\n")
		for _, row := range failures {
			builder.WriteString(fmt.Sprintf("- `%s`: %s\n", row.CaseID, strings.TrimSpace(row.Error)))
		}
	}
	return builder.String()
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
		if row.Status == "failed" || (row.Status == "passed" && !row.ScenarioPassed) {
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
