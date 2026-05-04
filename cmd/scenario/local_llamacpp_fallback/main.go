package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/teradakousuke/note_maker/internal/infrastructure/llamacpp"
)

const (
	defaultOutputDir = "tmp/local_llamacpp_fallback"
	defaultBaseURL   = "http://127.0.0.1:8081/v1"
	defaultModel     = "qwen3:30b-a3b"
)

func main() {
	started := time.Now()
	config, err := scenarioConfigFromEnv()
	if err != nil {
		fatalf("%v", err)
	}
	if err := os.MkdirAll(config.OutputDir, 0o755); err != nil {
		fatalf("create output dir: %v", err)
	}

	report := newPlanReport(config)
	endpoint := checkEndpoint(config)
	report.Endpoint = endpoint
	report.TotalElapsedSeconds = time.Since(started).Seconds()

	if !config.ExplicitRun {
		report.Status = "plan_only"
		report.Remaining = planRemaining(endpoint)
		writeReports(config.OutputDir, report)
		printPlan(report)
		return
	}
	if !endpoint.Available {
		report.Status = "plan_only"
		report.Remaining = []string{"Start or expose an already-approved local llama.cpp endpoint on the recorded loopback base URL, then rerun with RUN_LOCAL_LLAMACPP_FALLBACK_SCENARIO=1. This runner does not start llama-server or any other local model process."}
		writeReports(config.OutputDir, report)
		printPlan(report)
		return
	}

	report.Status = "running"
	writeReports(config.OutputDir, report)
	report.Commands = append(report.Commands, runStep(config, "author style", []string{
		"SCENARIO_OUTPUT_DIR=" + filepath.Join(config.OutputDir, "author_style"),
	}, "go", "run", "./cmd/scenario/author_style"))
	if lastCommandFailed(report.Commands) {
		finishWithFailure(config.OutputDir, started, report)
	}

	profileID := readProfileID(filepath.Join(config.OutputDir, "author_style", "profile.json"))
	report.Commands = append(report.Commands, runStep(config, "brief interview", []string{
		"SCENARIO_OUTPUT_DIR=" + filepath.Join(config.OutputDir, "brief_interview"),
		"STYLE_PROFILE_ID=" + profileID,
	}, "go", "run", "./cmd/scenario/brief_interview"))
	if lastCommandFailed(report.Commands) {
		finishWithFailure(config.OutputDir, started, report)
	}

	draftCommand := runStep(config, "draft generation", []string{
		"RUN_LOCAL_LLM_SCENARIO=1",
		"SCENARIO_OUTPUT_DIR=" + filepath.Join(config.OutputDir, "draft_generation"),
		"AUTHOR_PROFILE_PATH=" + filepath.Join(config.OutputDir, "author_style", "profile.json"),
		"WRITING_GUIDE_PATH=" + filepath.Join(config.OutputDir, "author_style", "guide.json"),
		"ARTICLE_BRIEF_PATH=" + filepath.Join(config.OutputDir, "brief_interview", "brief.json"),
	}, "go", "run", "./cmd/scenario/draft_generation")
	report.Commands = append(report.Commands, draftCommand)
	report.Result = resultFromDraftOutput(draftCommand.Stdout)
	enrichResultFromEvaluation(config, &report)
	report.Artifacts = scenarioArtifacts(config)
	report.TotalElapsedSeconds = time.Since(started).Seconds()

	if draftCommand.ExitCode != 0 {
		report.Status = "failed"
		report.Remaining = []string{"The dedicated local llama.cpp fallback run executed but did not meet the recorded thresholds. Inspect draft_generation artifacts before rerunning with a different model or llama-server load flags."}
		writeReports(config.OutputDir, report)
		fatalf("local llama.cpp fallback validation failed; report=%s", filepath.Join(config.OutputDir, "report.md"))
	}
	if !localFallbackPassed(report) {
		report.Status = "failed"
		report.Remaining = []string{"The local llama.cpp fallback response did not meet the recorded pass/fail thresholds."}
		writeReports(config.OutputDir, report)
		fatalf("local llama.cpp fallback validation did not pass thresholds; report=%s", filepath.Join(config.OutputDir, "report.md"))
	}

	report.Status = "passed"
	report.Remaining = nil
	writeReports(config.OutputDir, report)
	fmt.Printf("local llama.cpp fallback validation completed\n")
	fmt.Printf("status=%s\n", report.Status)
	fmt.Printf("base_url=%s\n", report.Runtime.BaseURL)
	fmt.Printf("model=%s\n", report.Runtime.Model)
	fmt.Printf("elapsed_seconds=%.2f\n", report.Result.ElapsedSeconds)
	fmt.Printf("score=%.1f\n", report.Result.Score)
	fmt.Printf("keyword_overlap=%d\n", report.Result.KeywordOverlap)
	fmt.Printf("runes=%d\n", report.Result.Runes)
	fmt.Printf("min_style_score=%.1f\n", report.Thresholds.MinStyleScore)
	fmt.Printf("min_keyword_overlap=%d\n", report.Thresholds.MinKeywordOverlap)
	fmt.Printf("min_draft_runes=%d\n", report.Thresholds.MinDraftRunes)
	fmt.Printf("load_flags=%s\n", report.Runtime.LoadFlags)
	fmt.Printf("report=%s\n", filepath.Join(config.OutputDir, "report.md"))
}

type scenarioConfig struct {
	OutputDir         string
	BaseURL           string
	Model             string
	VerifyModel       string
	LoadFlags         string
	MinStyleScore     float64
	MinKeywordOverlap int
	MinDraftRunes     int
	MaxAttempts       int
	TimeoutSeconds    int
	StreamDraft       bool
	ExplicitRun       bool
}

type scenarioReport struct {
	GeneratedBy         string          `json:"generated_by"`
	GeneratedAt         string          `json:"generated_at"`
	Status              string          `json:"status"`
	ExplicitRun         bool            `json:"explicit_run"`
	Runtime             runtimeReport   `json:"runtime"`
	Thresholds          thresholdReport `json:"thresholds"`
	Endpoint            endpointReport  `json:"endpoint"`
	Result              resultReport    `json:"result,omitempty"`
	Commands            []commandReport `json:"commands,omitempty"`
	Artifacts           artifactReport  `json:"artifacts,omitempty"`
	Remaining           []string        `json:"remaining,omitempty"`
	TotalElapsedSeconds float64         `json:"total_elapsed_seconds"`
}

type runtimeReport struct {
	BaseURL               string `json:"base_url"`
	Model                 string `json:"model"`
	VerifyModel           string `json:"verify_model"`
	LoadFlags             string `json:"load_flags"`
	LocalFallbackOnly     bool   `json:"local_fallback_only"`
	FallbackChainDisabled bool   `json:"fallback_chain_disabled"`
	StreamDraft           bool   `json:"stream_draft"`
	TimeoutSeconds        int    `json:"timeout_seconds"`
	MaxAttempts           int    `json:"max_attempts"`
}

type thresholdReport struct {
	MinStyleScore     float64 `json:"min_style_score"`
	MinKeywordOverlap int     `json:"min_keyword_overlap"`
	MinDraftRunes     int     `json:"min_draft_runes"`
}

type endpointReport struct {
	BaseURL   string   `json:"base_url"`
	Model     string   `json:"model"`
	Checked   bool     `json:"checked"`
	CheckedAt string   `json:"checked_at"`
	Available bool     `json:"available"`
	Models    []string `json:"models,omitempty"`
	Error     string   `json:"error,omitempty"`
}

type resultReport struct {
	ScenarioPassed        bool    `json:"scenario_passed"`
	Attempt               int     `json:"attempt"`
	SelectedAttempt       int     `json:"selected_attempt"`
	Passed                bool    `json:"passed"`
	Score                 float64 `json:"score"`
	KeywordOverlap        int     `json:"keyword_overlap"`
	Runes                 int     `json:"runes"`
	VerificationPerformed bool    `json:"verification_performed"`
	VerificationPassed    bool    `json:"verification_passed"`
	ElapsedSeconds        float64 `json:"elapsed_seconds"`
	Streaming             bool    `json:"streaming"`
	FirstChunkMs          int     `json:"first_chunk_ms,omitempty"`
	Chunks                int     `json:"chunks,omitempty"`
}

type commandReport struct {
	Label          string  `json:"label"`
	Command        string  `json:"command"`
	ExitCode       int     `json:"exit_code"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	StdoutPath     string  `json:"stdout_path"`
	StderrPath     string  `json:"stderr_path"`
	Stdout         string  `json:"-"`
}

type artifactReport struct {
	Report       string `json:"report"`
	JSON         string `json:"json"`
	Draft        string `json:"draft,omitempty"`
	Evaluation   string `json:"evaluation,omitempty"`
	Verification string `json:"verification,omitempty"`
}

func scenarioConfigFromEnv() (scenarioConfig, error) {
	config := scenarioConfig{
		OutputDir:         envOrDefault("LOCAL_LLAMACPP_FALLBACK_OUTPUT_DIR", envOrDefault("SCENARIO_OUTPUT_DIR", defaultOutputDir)),
		BaseURL:           envOrDefault("LOCAL_LLAMACPP_FALLBACK_BASE_URL", defaultBaseURL),
		Model:             envOrDefault("LOCAL_LLAMACPP_FALLBACK_MODEL", defaultModel),
		VerifyModel:       envOrDefault("LOCAL_LLAMACPP_FALLBACK_VERIFY_MODEL", envOrDefault("LOCAL_LLAMACPP_FALLBACK_MODEL", defaultModel)),
		LoadFlags:         envOrDefault("LOCAL_LLAMACPP_FALLBACK_LOAD_FLAGS", envOrDefault("LLAMACPP_LOAD_FLAGS", "")),
		MinStyleScore:     envFloat("LOCAL_LLAMACPP_FALLBACK_MIN_STYLE_SCORE", envFloat("SCENARIO_MIN_STYLE_SCORE", 82)),
		MinKeywordOverlap: envInt("LOCAL_LLAMACPP_FALLBACK_MIN_KEYWORD_OVERLAP", 70),
		MinDraftRunes:     envInt("LOCAL_LLAMACPP_FALLBACK_MIN_DRAFT_RUNES", envInt("SCENARIO_MIN_DRAFT_RUNES", 2800)),
		MaxAttempts:       envInt("LOCAL_LLAMACPP_FALLBACK_MAX_ATTEMPTS", envInt("DRAFT_MAX_ATTEMPTS", 3)),
		TimeoutSeconds:    envInt("LOCAL_LLAMACPP_FALLBACK_TIMEOUT_SECONDS", envInt("LLM_TIMEOUT_SECONDS", 900)),
		StreamDraft:       envOrDefault("LOCAL_LLAMACPP_FALLBACK_STREAM_DRAFT", "1") == "1",
		ExplicitRun:       os.Getenv("RUN_LOCAL_LLAMACPP_FALLBACK_SCENARIO") == "1",
	}
	if err := requireLoopbackBaseURL(config.BaseURL); err != nil {
		return scenarioConfig{}, err
	}
	return config, nil
}

func requireLoopbackBaseURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("parse LOCAL_LLAMACPP_FALLBACK_BASE_URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("LOCAL_LLAMACPP_FALLBACK_BASE_URL must use http or https")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("LOCAL_LLAMACPP_FALLBACK_BASE_URL must include a host")
	}
	if strings.Contains(strings.ToLower(host), "evo-x2") {
		return fmt.Errorf("LOCAL_LLAMACPP_FALLBACK_BASE_URL must be local fallback only, got Evo X2 host %q", host)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("LOCAL_LLAMACPP_FALLBACK_BASE_URL must be a loopback llama.cpp endpoint, got %q", host)
}

func newPlanReport(config scenarioConfig) scenarioReport {
	return scenarioReport{
		GeneratedBy: "cmd/scenario/local_llamacpp_fallback",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Status:      "plan_only",
		ExplicitRun: config.ExplicitRun,
		Runtime: runtimeReport{
			BaseURL:               config.BaseURL,
			Model:                 config.Model,
			VerifyModel:           config.VerifyModel,
			LoadFlags:             config.LoadFlags,
			LocalFallbackOnly:     true,
			FallbackChainDisabled: true,
			StreamDraft:           config.StreamDraft,
			TimeoutSeconds:        config.TimeoutSeconds,
			MaxAttempts:           config.MaxAttempts,
		},
		Thresholds: thresholdReport{
			MinStyleScore:     config.MinStyleScore,
			MinKeywordOverlap: config.MinKeywordOverlap,
			MinDraftRunes:     config.MinDraftRunes,
		},
		Artifacts: scenarioArtifacts(config),
	}
}

func checkEndpoint(config scenarioConfig) endpointReport {
	report := endpointReport{
		BaseURL:   config.BaseURL,
		Model:     config.Model,
		Checked:   true,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, err := llamacpp.NewClient(config.BaseURL, config.Model, nil)
	if err != nil {
		report.Error = err.Error()
		return report
	}
	models, err := client.ListModels(ctx)
	if err != nil {
		report.Error = err.Error()
		return report
	}
	report.Available = true
	report.Models = models
	return report
}

func runStep(config scenarioConfig, label string, extraEnv []string, name string, args ...string) commandReport {
	fmt.Printf("running %s...\n", label)
	started := time.Now()
	command := exec.Command(name, args...)
	command.Env = commandEnv(config, extraEnv)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	exitCode := 0
	if err != nil {
		exitCode = 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	stdoutPath := filepath.Join(config.OutputDir, "logs", sanitizeLabel(label)+".stdout")
	stderrPath := filepath.Join(config.OutputDir, "logs", sanitizeLabel(label)+".stderr")
	mustWriteFile(stdoutPath, stdout.String())
	mustWriteFile(stderrPath, stderr.String())
	if stdout.Len() > 0 {
		fmt.Print(stdout.String())
	}
	if stderr.Len() > 0 {
		fmt.Fprint(os.Stderr, stderr.String())
	}
	return commandReport{
		Label:          label,
		Command:        strings.Join(append([]string{name}, args...), " "),
		ExitCode:       exitCode,
		ElapsedSeconds: time.Since(started).Seconds(),
		StdoutPath:     stdoutPath,
		StderrPath:     stderrPath,
		Stdout:         stdout.String(),
	}
}

func commandEnv(config scenarioConfig, extra []string) []string {
	values := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	overrides := []string{
		"NOTE_MAKER_SKIP_ENV=1",
		"LLM_BASE_URL=" + config.BaseURL,
		"LLAMACPP_BASE_URL=" + config.BaseURL,
		"LLM_MODEL=" + config.Model,
		"STYLE_LLM_MODEL=" + config.Model,
		"BRIEF_LLM_MODEL=" + config.Model,
		"ARTICLE_LLM_MODEL=" + config.Model,
		"DRAFT_LLM_MODEL=" + config.Model,
		"VERIFY_LLM_MODEL=" + config.VerifyModel,
		"LLM_TIMEOUT_SECONDS=" + strconv.Itoa(config.TimeoutSeconds),
		"SCENARIO_DRAFT_TIMEOUT_SECONDS=" + strconv.Itoa(config.TimeoutSeconds),
		"SCENARIO_MIN_STYLE_SCORE=" + fmt.Sprintf("%.1f", config.MinStyleScore),
		"SCENARIO_MIN_KEYWORD_OVERLAP=" + strconv.Itoa(config.MinKeywordOverlap),
		"SCENARIO_MIN_DRAFT_RUNES=" + strconv.Itoa(config.MinDraftRunes),
		"DRAFT_MAX_ATTEMPTS=" + strconv.Itoa(config.MaxAttempts),
		"SCENARIO_STREAM_DRAFT=" + boolEnv(config.StreamDraft),
		"DRAFT_LLM_MAX_TOKENS=" + envOrDefault("LOCAL_LLAMACPP_FALLBACK_MAX_TOKENS", envOrDefault("DRAFT_LLM_MAX_TOKENS", "4096")),
		"DRAFT_LLM_TEMPERATURE=" + envOrDefault("LOCAL_LLAMACPP_FALLBACK_TEMPERATURE", envOrDefault("DRAFT_LLM_TEMPERATURE", "0.8")),
		"DRAFT_LLM_TOP_P=" + envOrDefault("LOCAL_LLAMACPP_FALLBACK_TOP_P", envOrDefault("DRAFT_LLM_TOP_P", "0.9")),
		"SCENARIO_VERIFY_DRAFT=" + envOrDefault("LOCAL_LLAMACPP_FALLBACK_VERIFY_DRAFT", envOrDefault("SCENARIO_VERIFY_DRAFT", "0")),
		"LLM_FALLBACK_BASE_URLS=",
		"FALLBACK_LLM_BASE_URLS=",
		"FALLBACK_LLM_BASE_URL=",
		"FALLBACK_LLAMACPP_BASE_URL=",
		"STYLE_LLM_FALLBACK_BASE_URLS=",
		"STYLE_FALLBACK_LLM_BASE_URLS=",
		"STYLE_FALLBACK_LLM_BASE_URL=",
		"BRIEF_LLM_FALLBACK_BASE_URLS=",
		"BRIEF_FALLBACK_LLM_BASE_URLS=",
		"BRIEF_FALLBACK_LLM_BASE_URL=",
		"ARTICLE_LLM_FALLBACK_BASE_URLS=",
		"ARTICLE_FALLBACK_LLM_BASE_URLS=",
		"ARTICLE_FALLBACK_LLM_BASE_URL=",
		"DRAFT_LLM_FALLBACK_BASE_URLS=",
		"DRAFT_FALLBACK_LLM_BASE_URLS=",
		"DRAFT_FALLBACK_LLM_BASE_URL=",
		"VERIFY_LLM_FALLBACK_BASE_URLS=",
		"VERIFY_FALLBACK_LLM_BASE_URLS=",
		"VERIFY_FALLBACK_LLM_BASE_URL=",
		"STYLE_LLM_FALLBACK_MODELS=",
		"STYLE_FALLBACK_LLM_MODELS=",
		"STYLE_FALLBACK_LLM_MODEL=",
		"BRIEF_LLM_FALLBACK_MODELS=",
		"BRIEF_FALLBACK_LLM_MODELS=",
		"BRIEF_FALLBACK_LLM_MODEL=",
		"ARTICLE_LLM_FALLBACK_MODELS=",
		"ARTICLE_FALLBACK_LLM_MODELS=",
		"ARTICLE_FALLBACK_LLM_MODEL=",
		"DRAFT_LLM_FALLBACK_MODELS=",
		"DRAFT_FALLBACK_LLM_MODELS=",
		"DRAFT_FALLBACK_LLM_MODEL=",
		"VERIFY_LLM_FALLBACK_MODELS=",
		"VERIFY_FALLBACK_LLM_MODELS=",
		"VERIFY_FALLBACK_LLM_MODEL=",
	}
	overrides = append(overrides, extra...)
	for _, entry := range overrides {
		key, value, _ := strings.Cut(entry, "=")
		values[key] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}

func resultFromDraftOutput(output string) resultReport {
	values := keyValueLines(output)
	return resultReport{
		ScenarioPassed:        parseBool(values["scenario_passed"]),
		Attempt:               parseInt(values["attempt"]),
		SelectedAttempt:       parseInt(values["selected_attempt"]),
		Passed:                parseBool(values["passed"]),
		Score:                 parseFloat(values["score"]),
		KeywordOverlap:        parseInt(values["keyword_overlap"]),
		Runes:                 parseInt(values["runes"]),
		VerificationPerformed: parseBool(values["verification_performed"]),
		VerificationPassed:    parseBool(values["verification_passed"]),
		ElapsedSeconds:        parseFloat(values["elapsed_seconds"]),
		Streaming:             parseBool(values["streaming"]),
		FirstChunkMs:          parseInt(values["first_chunk_ms"]),
		Chunks:                parseInt(values["chunks"]),
	}
}

func enrichResultFromEvaluation(config scenarioConfig, report *scenarioReport) {
	evaluationPath := filepath.Join(config.OutputDir, "draft_generation", "evaluation.json")
	encoded, err := os.ReadFile(evaluationPath)
	if err != nil {
		return
	}
	var evaluation struct {
		Comparison struct {
			MetricScores map[string]int `json:"metric_scores"`
		} `json:"comparison"`
	}
	if err := json.Unmarshal(encoded, &evaluation); err != nil {
		return
	}
	report.Result.KeywordOverlap = evaluation.Comparison.MetricScores["keyword_overlap"]
}

func localFallbackPassed(report scenarioReport) bool {
	return report.Result.ScenarioPassed &&
		report.Result.Score >= report.Thresholds.MinStyleScore &&
		report.Result.KeywordOverlap >= report.Thresholds.MinKeywordOverlap &&
		report.Result.Runes >= report.Thresholds.MinDraftRunes &&
		(!report.Result.VerificationPerformed || report.Result.VerificationPassed)
}

func keyValueLines(output string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		values[key] = strings.TrimSpace(value)
	}
	return values
}

func readProfileID(path string) string {
	encoded, err := os.ReadFile(path)
	if err != nil {
		fatalf("read profile: %v", err)
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		fatalf("decode profile: %v", err)
	}
	if strings.TrimSpace(payload.ID) == "" {
		fatalf("profile id was empty")
	}
	return payload.ID
}

func lastCommandFailed(commands []commandReport) bool {
	if len(commands) == 0 {
		return false
	}
	return commands[len(commands)-1].ExitCode != 0
}

func finishWithFailure(outputDir string, started time.Time, report scenarioReport) {
	report.Status = "failed"
	report.TotalElapsedSeconds = time.Since(started).Seconds()
	report.Remaining = []string{"A setup step failed before draft generation. Inspect the command stdout/stderr files recorded in the report."}
	writeReports(outputDir, report)
	fatalf("local llama.cpp fallback scenario setup failed; report=%s", filepath.Join(outputDir, "report.md"))
}

func planRemaining(endpoint endpointReport) []string {
	remaining := []string{"Set RUN_LOCAL_LLAMACPP_FALLBACK_SCENARIO=1 before running live validation."}
	if !endpoint.Available {
		remaining = append(remaining, "Expose an already-approved local llama.cpp OpenAI-compatible /v1 endpoint on the recorded loopback base URL. This runner does not start llama-server or Ollama.")
	}
	return remaining
}

func writeReports(outputDir string, report scenarioReport) {
	report.Artifacts.Report = filepath.Join(outputDir, "report.md")
	report.Artifacts.JSON = filepath.Join(outputDir, "report.json")
	mustWriteJSON(filepath.Join(outputDir, "report.json"), report)
	mustWriteFile(filepath.Join(outputDir, "report.md"), markdownReport(report))
}

func scenarioArtifacts(config scenarioConfig) artifactReport {
	draftDir := filepath.Join(config.OutputDir, "draft_generation")
	return artifactReport{
		Report:       filepath.Join(config.OutputDir, "report.md"),
		JSON:         filepath.Join(config.OutputDir, "report.json"),
		Draft:        filepath.Join(draftDir, "draft.md"),
		Evaluation:   filepath.Join(draftDir, "evaluation.json"),
		Verification: filepath.Join(draftDir, "verification.json"),
	}
}

func markdownReport(report scenarioReport) string {
	var builder strings.Builder
	builder.WriteString("# Local llama.cpp fallback validation\n\n")
	builder.WriteString(fmt.Sprintf("- Generated by: `%s`\n", report.GeneratedBy))
	builder.WriteString(fmt.Sprintf("- Generated at: `%s`\n", report.GeneratedAt))
	builder.WriteString(fmt.Sprintf("- Status: `%s`\n", report.Status))
	builder.WriteString(fmt.Sprintf("- Explicit run: `%v`\n", report.ExplicitRun))
	builder.WriteString("\n## Runtime\n\n")
	builder.WriteString(fmt.Sprintf("- Base URL: `%s`\n", report.Runtime.BaseURL))
	builder.WriteString(fmt.Sprintf("- Model: `%s`\n", report.Runtime.Model))
	builder.WriteString(fmt.Sprintf("- Verify model: `%s`\n", report.Runtime.VerifyModel))
	builder.WriteString(fmt.Sprintf("- Load flags: `%s`\n", valueOrNone(report.Runtime.LoadFlags)))
	builder.WriteString(fmt.Sprintf("- Local fallback only: `%v`\n", report.Runtime.LocalFallbackOnly))
	builder.WriteString(fmt.Sprintf("- Fallback chain disabled: `%v`\n", report.Runtime.FallbackChainDisabled))
	builder.WriteString(fmt.Sprintf("- Streaming: `%v`\n", report.Runtime.StreamDraft))
	builder.WriteString(fmt.Sprintf("- Timeout seconds: `%d`\n", report.Runtime.TimeoutSeconds))
	builder.WriteString(fmt.Sprintf("- Max attempts: `%d`\n", report.Runtime.MaxAttempts))
	builder.WriteString("\n## Thresholds\n\n")
	builder.WriteString(fmt.Sprintf("- Min style score: `%.1f`\n", report.Thresholds.MinStyleScore))
	builder.WriteString(fmt.Sprintf("- Min keyword overlap: `%d`\n", report.Thresholds.MinKeywordOverlap))
	builder.WriteString(fmt.Sprintf("- Min draft runes: `%d`\n", report.Thresholds.MinDraftRunes))
	builder.WriteString("\n## Endpoint\n\n")
	builder.WriteString(fmt.Sprintf("- Checked: `%v`\n", report.Endpoint.Checked))
	builder.WriteString(fmt.Sprintf("- Available: `%v`\n", report.Endpoint.Available))
	if report.Endpoint.Error != "" {
		builder.WriteString(fmt.Sprintf("- Error: `%s`\n", report.Endpoint.Error))
	}
	if len(report.Endpoint.Models) > 0 {
		builder.WriteString(fmt.Sprintf("- Models: `%s`\n", strings.Join(report.Endpoint.Models, ", ")))
	}
	if report.Result.Score > 0 || report.Result.Runes > 0 || report.Result.ElapsedSeconds > 0 {
		builder.WriteString("\n## Result\n\n")
		builder.WriteString(fmt.Sprintf("- Scenario passed: `%v`\n", report.Result.ScenarioPassed))
		builder.WriteString(fmt.Sprintf("- Elapsed seconds: `%.2f`\n", report.Result.ElapsedSeconds))
		builder.WriteString(fmt.Sprintf("- Score: `%.1f / %.1f`\n", report.Result.Score, report.Thresholds.MinStyleScore))
		builder.WriteString(fmt.Sprintf("- Keyword overlap: `%d / %d`\n", report.Result.KeywordOverlap, report.Thresholds.MinKeywordOverlap))
		builder.WriteString(fmt.Sprintf("- Runes: `%d / %d`\n", report.Result.Runes, report.Thresholds.MinDraftRunes))
		builder.WriteString(fmt.Sprintf("- Verification: `performed=%v passed=%v`\n", report.Result.VerificationPerformed, report.Result.VerificationPassed))
	}
	if len(report.Commands) > 0 {
		builder.WriteString("\n## Commands\n\n")
		for _, command := range report.Commands {
			builder.WriteString(fmt.Sprintf("- `%s`: exit `%d`, elapsed `%.2fs`, stdout `%s`, stderr `%s`\n", command.Label, command.ExitCode, command.ElapsedSeconds, command.StdoutPath, command.StderrPath))
		}
	}
	if len(report.Remaining) > 0 {
		builder.WriteString("\n## Remaining\n\n")
		for _, item := range report.Remaining {
			builder.WriteString("- " + item + "\n")
		}
	}
	builder.WriteString("\n## Artifacts\n\n")
	builder.WriteString(fmt.Sprintf("- JSON: `%s`\n", report.Artifacts.JSON))
	builder.WriteString(fmt.Sprintf("- Draft: `%s`\n", report.Artifacts.Draft))
	builder.WriteString(fmt.Sprintf("- Evaluation: `%s`\n", report.Artifacts.Evaluation))
	builder.WriteString(fmt.Sprintf("- Verification: `%s`\n", report.Artifacts.Verification))
	return builder.String()
}

func printPlan(report scenarioReport) {
	fmt.Printf("local llama.cpp fallback validation plan recorded\n")
	fmt.Printf("status=%s\n", report.Status)
	fmt.Printf("explicit_run=%v\n", report.ExplicitRun)
	fmt.Printf("endpoint_available=%v\n", report.Endpoint.Available)
	if report.Endpoint.Error != "" {
		fmt.Printf("endpoint_error=%s\n", report.Endpoint.Error)
	}
	fmt.Printf("base_url=%s\n", report.Runtime.BaseURL)
	fmt.Printf("model=%s\n", report.Runtime.Model)
	fmt.Printf("min_style_score=%.1f\n", report.Thresholds.MinStyleScore)
	fmt.Printf("min_keyword_overlap=%d\n", report.Thresholds.MinKeywordOverlap)
	fmt.Printf("min_draft_runes=%d\n", report.Thresholds.MinDraftRunes)
	fmt.Printf("load_flags=%s\n", report.Runtime.LoadFlags)
	fmt.Printf("report=%s\n", report.Artifacts.Report)
}

func mustWriteJSON(path string, value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatalf("encode %s: %v", path, err)
	}
	mustWriteFile(path, string(encoded)+"\n")
}

func mustWriteFile(path string, content string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fatalf("create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fatalf("write %s: %v", path, err)
	}
}

func sanitizeLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('_')
	}
	return strings.Trim(builder.String(), "_")
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseBool(value string) bool {
	parsed, _ := strconv.ParseBool(strings.TrimSpace(value))
	return parsed
}

func parseInt(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}

func parseFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func boolEnv(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func valueOrNone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(not recorded)"
	}
	return value
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
