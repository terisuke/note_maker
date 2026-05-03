package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	authorstyleapp "github.com/teradakousuke/note_maker/internal/application/authorstyle"
	briefapp "github.com/teradakousuke/note_maker/internal/application/brief"
	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
	"github.com/teradakousuke/note_maker/internal/infrastructure/llamacpp"
	"github.com/teradakousuke/note_maker/internal/infrastructure/repository/memory"
	sqliterepo "github.com/teradakousuke/note_maker/internal/infrastructure/repository/sqlite"
	sourcefetch "github.com/teradakousuke/note_maker/internal/infrastructure/source"
)

var workflowStore = newWorkflowStore()

type workflowStoreBackend interface {
	SaveAuthorStyle(authorstyleapp.AnalyzeResult) error
	GetAuthorStyle(string) (authorstyleapp.AnalyzeResult, bool)
	ListAuthorStyles() ([]authorstyleapp.AnalyzeResult, error)
	SaveSession(briefdomain.ArticleBriefSession) error
	GetSession(string) (briefdomain.ArticleBriefSession, bool)
	ListSessions() ([]briefdomain.ArticleBriefSession, error)
	SaveBrief(string, briefdomain.ArticleBrief) error
	GetBrief(string) (briefdomain.ArticleBrief, bool)
	ListBriefs() (map[string]briefdomain.ArticleBrief, error)
	ListBriefVersions(string) ([]briefdomain.ArticleBriefVersion, error)
	SavePersona(personadomain.Persona) error
	GetPersona(string) (personadomain.Persona, bool)
	ListPersonas() ([]personadomain.Persona, error)
	DeletePersona(string) error
	GetProfileAndGuide(string) (authordomain.AuthorStyleProfile, authordomain.WritingStyleGuide, bool)
}

type workflowHistoryReader interface {
	GetProject(string) (sqliterepo.ProjectRecord, bool)
	ListProjects() ([]sqliterepo.ProjectRecord, error)
	GetArticle(string) (sqliterepo.ArticleRecord, bool)
	ListArticlesByProject(string) ([]sqliterepo.ArticleRecord, error)
	ListSourceSnapshots(string, string) ([]sqliterepo.SourceSnapshotRecord, error)
	GetDraft(string) (sqliterepo.DraftRecord, bool)
	ListDrafts(string) ([]sqliterepo.DraftRecord, error)
	ListSectionRegenerations(string) ([]sqliterepo.SectionRegenerationRecord, error)
}

type workflowHistoryWriter interface {
	workflowHistoryReader
	SaveProject(sqliterepo.ProjectRecord) error
	SaveArticle(sqliterepo.ArticleRecord) error
	SaveDraft(sqliterepo.DraftRecord) error
	SaveSectionRegeneration(sqliterepo.SectionRegenerationRecord) error
}

func newWorkflowStore() workflowStoreBackend {
	config := resolveWorkflowStorageConfig()
	setActiveWorkflowStorage(config)
	if config.Driver == storageDriverSQLite {
		path := config.Path
		store, err := sqliterepo.NewWorkflowStore(path)
		if err == nil {
			return store
		}
		panic(fmt.Sprintf("initialize sqlite workflow store: %v", err))
	}
	path := config.Path
	store, err := memory.NewPersistentWorkflowStore(path)
	if err == nil {
		return store
	}
	return memory.NewWorkflowStore()
}

type analyzeAuthorStyleRequest struct {
	Username       string   `json:"username"`
	SourceSelector string   `json:"source_selector"`
	ArticleURLs    []string `json:"article_urls"`
	Limit          int      `json:"limit"`
	StyleModel     string   `json:"style_model"`
	PersonaID      string   `json:"persona_id"`
	OutputFormatID string   `json:"output_format_id"`
}

type seedAuthorStyleRequest struct {
	PersonaID      string `json:"persona_id"`
	OutputFormatID string `json:"output_format_id"`
}

type createPersonaRequest struct {
	ID            string                       `json:"id"`
	DisplayName   string                       `json:"display_name"`
	Description   string                       `json:"description"`
	DefaultFormat string                       `json:"default_format"`
	Sources       []personadomain.AuthorSource `json:"sources"`
	VoiceNotes    personadomain.VoiceNotes     `json:"voice_notes"`
}

type updatePersonaRequest struct {
	ID            string                        `json:"id"`
	DisplayName   *string                       `json:"display_name"`
	Description   *string                       `json:"description"`
	DefaultFormat *string                       `json:"default_format"`
	Sources       *[]personadomain.AuthorSource `json:"sources"`
	VoiceNotes    *personadomain.VoiceNotes     `json:"voice_notes"`
}

type authorStyleResponse struct {
	ID            string `json:"id"`
	ProfileID     string `json:"profile_id"`
	GuideID       string `json:"guide_id"`
	GuideMarkdown string `json:"guide_markdown"`
	ArticleCount  int    `json:"article_count"`
	CreatedAt     string `json:"created_at,omitempty"`
	Source        any    `json:"source"`
	Profile       any    `json:"profile"`
	Guide         any    `json:"guide"`
}

type authorStyleListResponse struct {
	StyleGuides []styleGuideArtifactResponse `json:"style_guides"`
}

type styleGuideArtifactResponse struct {
	ID            string                          `json:"id"`
	AnalysisID    string                          `json:"analysis_id"`
	ProfileID     string                          `json:"profile_id"`
	GuideID       string                          `json:"guide_id"`
	Title         string                          `json:"title"`
	Description   string                          `json:"description,omitempty"`
	CreatedAt     string                          `json:"created_at,omitempty"`
	ArticleCount  int                             `json:"article_count"`
	GuideMarkdown string                          `json:"guide_markdown"`
	Source        authordomain.AuthorSource       `json:"source"`
	Profile       authordomain.AuthorStyleProfile `json:"profile"`
	Guide         authordomain.WritingStyleGuide  `json:"guide"`
}

type createBriefSessionRequest struct {
	StyleProfileID string                `json:"style_profile_id"`
	SessionID      string                `json:"session_id"`
	PersonaID      string                `json:"persona_id"`
	OutputFormatID string                `json:"output_format_id"`
	BriefModel     string                `json:"brief_model"`
	Questions      []articleQuestionJSON `json:"questions"`
}

type answerBriefSessionRequest struct {
	Content      string `json:"content"`
	SkipDeepDive bool   `json:"skip_deep_dive"`
	BriefModel   string `json:"brief_model"`
}

type editBriefAnswerRequest struct {
	Content    string `json:"content"`
	BriefModel string `json:"brief_model"`
}

type updateStyleGuideRequest struct {
	GuideMarkdown        string   `json:"guide_markdown"`
	PreferredFirstPerson string   `json:"preferred_first_person"`
	RecurringThemes      []string `json:"recurring_themes"`
	ParagraphRhythm      string   `json:"paragraph_rhythm"`
	SentenceRhythm       string   `json:"sentence_rhythm"`
	HeadingGuidance      string   `json:"heading_guidance"`
	QuoteGuidance        string   `json:"quote_guidance"`
	OpeningPatterns      []string `json:"opening_patterns"`
	ConclusionPatterns   []string `json:"conclusion_patterns"`
	Warnings             []string `json:"warnings"`
}

type updateBriefResponse struct {
	Brief briefArtifactResponse `json:"brief"`
}

type briefSessionResponse struct {
	SessionID       string                    `json:"session_id"`
	StyleProfileID  string                    `json:"style_profile_id"`
	PersonaID       string                    `json:"persona_id"`
	OutputFormatID  string                    `json:"output_format_id"`
	ParentSessionID string                    `json:"parent_session_id,omitempty"`
	Phase           string                    `json:"phase"`
	Completed       bool                      `json:"completed"`
	NextQuestion    *articleQuestionJSON      `json:"next_question,omitempty"`
	Brief           *briefdomain.ArticleBrief `json:"brief,omitempty"`
	Answers         []briefdomain.BriefAnswer `json:"answers"`
}

type briefSessionListResponse struct {
	Sessions []briefSessionSummaryResponse `json:"sessions"`
}

type briefSessionSummaryResponse struct {
	SessionID       string `json:"session_id"`
	StyleProfileID  string `json:"style_profile_id"`
	PersonaID       string `json:"persona_id"`
	OutputFormatID  string `json:"output_format_id"`
	ParentSessionID string `json:"parent_session_id,omitempty"`
	Phase           string `json:"phase"`
	Completed       bool   `json:"completed"`
	AnswerCount     int    `json:"answer_count"`
	QuestionCount   int    `json:"question_count"`
	BriefAvailable  bool   `json:"brief_available"`
	Title           string `json:"title,omitempty"`
}

type briefArtifactListResponse struct {
	Briefs []briefArtifactResponse `json:"briefs"`
}

type briefVersionListResponse struct {
	SessionID      string                 `json:"session_id"`
	CurrentVersion int                    `json:"current_version"`
	Versions       []briefVersionResponse `json:"versions"`
}

type briefVersionResponse struct {
	SessionID      string                   `json:"session_id"`
	Version        int                      `json:"version"`
	Current        bool                     `json:"current"`
	CreatedAt      string                   `json:"created_at,omitempty"`
	Title          string                   `json:"title,omitempty"`
	StyleProfileID string                   `json:"style_profile_id,omitempty"`
	PersonaID      string                   `json:"persona_id,omitempty"`
	OutputFormatID string                   `json:"output_format_id,omitempty"`
	Brief          briefdomain.ArticleBrief `json:"brief"`
}

type briefArtifactResponse struct {
	SessionID       string                   `json:"session_id"`
	StyleProfileID  string                   `json:"style_profile_id"`
	PersonaID       string                   `json:"persona_id"`
	OutputFormatID  string                   `json:"output_format_id"`
	ParentSessionID string                   `json:"parent_session_id,omitempty"`
	Title           string                   `json:"title"`
	Description     string                   `json:"description,omitempty"`
	AnswerCount     int                      `json:"answer_count"`
	DeepDiveCount   int                      `json:"deep_dive_count"`
	Brief           briefdomain.ArticleBrief `json:"brief"`
}

type workflowArtifactsResponse struct {
	StyleGuides []styleGuideArtifactResponse  `json:"style_guides"`
	Sessions    []briefSessionSummaryResponse `json:"sessions"`
	Briefs      []briefArtifactResponse       `json:"briefs"`
	Projects    []projectHistoryResponse      `json:"projects,omitempty"`
	Articles    []articleHistoryResponse      `json:"articles,omitempty"`
	Drafts      []draftHistoryResponse        `json:"drafts,omitempty"`
}

type projectHistoryListResponse struct {
	Projects []projectHistoryResponse `json:"projects"`
}

type projectHistoryDetailResponse struct {
	Project         projectHistoryResponse          `json:"project"`
	Articles        []articleHistoryResponse        `json:"articles"`
	SourceSnapshots []sourceSnapshotHistoryResponse `json:"source_snapshots"`
}

type articleHistoryDetailResponse struct {
	Article         articleHistoryResponse          `json:"article"`
	Drafts          []draftHistoryResponse          `json:"drafts"`
	SourceSnapshots []sourceSnapshotHistoryResponse `json:"source_snapshots"`
}

type draftHistoryDetailResponse struct {
	Draft                draftHistoryResponse                 `json:"draft"`
	SourceSnapshots      []sourceSnapshotHistoryResponse      `json:"source_snapshots"`
	SectionRegenerations []sectionRegenerationHistoryResponse `json:"section_regenerations"`
}

type projectHistoryResponse struct {
	ID              string                          `json:"id"`
	Name            string                          `json:"name"`
	CreatedAt       string                          `json:"created_at,omitempty"`
	UpdatedAt       string                          `json:"updated_at,omitempty"`
	Metadata        map[string]any                  `json:"metadata,omitempty"`
	Articles        []articleHistoryResponse        `json:"articles,omitempty"`
	SourceSnapshots []sourceSnapshotHistoryResponse `json:"source_snapshots,omitempty"`
}

type articleHistoryResponse struct {
	ID              string                          `json:"id"`
	ProjectID       string                          `json:"project_id,omitempty"`
	PersonaID       string                          `json:"persona_id"`
	OutputFormatID  string                          `json:"output_format_id"`
	BriefSessionID  string                          `json:"brief_session_id,omitempty"`
	CurrentDraftID  string                          `json:"current_draft_id,omitempty"`
	Title           string                          `json:"title,omitempty"`
	CreatedAt       string                          `json:"created_at,omitempty"`
	UpdatedAt       string                          `json:"updated_at,omitempty"`
	Metadata        map[string]any                  `json:"metadata,omitempty"`
	Drafts          []draftHistoryResponse          `json:"drafts,omitempty"`
	SourceSnapshots []sourceSnapshotHistoryResponse `json:"source_snapshots,omitempty"`
}

type sourceSnapshotHistoryResponse struct {
	ID          string `json:"id"`
	ScopeType   string `json:"scope_type"`
	ScopeID     string `json:"scope_id"`
	Selector    any    `json:"selector"`
	Profile     any    `json:"profile,omitempty"`
	Article     any    `json:"article,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
	FetchedAt   string `json:"fetched_at,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

type draftHistoryResponse struct {
	ID                      string                               `json:"id"`
	ArticleID               string                               `json:"article_id,omitempty"`
	SessionID               string                               `json:"session_id,omitempty"`
	StyleProfileID          string                               `json:"style_profile_id,omitempty"`
	PersonaID               string                               `json:"persona_id,omitempty"`
	OutputFormatID          string                               `json:"output_format_id,omitempty"`
	Version                 int                                  `json:"version"`
	Markdown                string                               `json:"markdown"`
	ContentHash             string                               `json:"content_hash,omitempty"`
	Evaluation              draftapp.StyleEvaluation             `json:"evaluation"`
	Verification            draftapp.FinalVerification           `json:"verification"`
	QuestionTemplateVersion string                               `json:"question_template_version,omitempty"`
	CreatedAt               string                               `json:"created_at,omitempty"`
	SourceSnapshots         []sourceSnapshotHistoryResponse      `json:"source_snapshots,omitempty"`
	SectionRegenerations    []sectionRegenerationHistoryResponse `json:"section_regenerations,omitempty"`
}

type sectionRegenerationHistoryResponse struct {
	ID                   string                     `json:"id"`
	DraftID              string                     `json:"draft_id"`
	ArticleID            string                     `json:"article_id,omitempty"`
	SectionAnchor        string                     `json:"section_anchor"`
	SectionHeading       string                     `json:"section_heading,omitempty"`
	BaseVersion          int                        `json:"base_version"`
	Version              int                        `json:"version"`
	ReplacementMarkdown  string                     `json:"replacement_markdown"`
	UpdatedDraftMarkdown string                     `json:"updated_draft_markdown"`
	UpdatedContentHash   string                     `json:"updated_content_hash,omitempty"`
	Verification         draftapp.FinalVerification `json:"verification"`
	CreatedAt            string                     `json:"created_at,omitempty"`
}

type briefSessionTemplateResponse struct {
	PersonaID      string                `json:"persona_id"`
	OutputFormatID string                `json:"output_format_id"`
	Questions      []articleQuestionJSON `json:"questions"`
}

type articleQuestionJSON struct {
	ID               string `json:"id"`
	Text             string `json:"text"`
	FlowType         string `json:"flow_type"`
	TargetField      string `json:"target_field,omitempty"`
	TargetQuestionID string `json:"target_question_id,omitempty"`
	FollowUpIndex    int    `json:"follow_up_index,omitempty"`
}

type generateDraftRequest struct {
	StyleProfileID string `json:"style_profile_id"`
	SessionID      string `json:"session_id"`
	PersonaID      string `json:"persona_id"`
	OutputFormatID string `json:"output_format_id"`
	DraftModel     string `json:"draft_model"`
	VerifyModel    string `json:"verify_model"`
}

type regenerateDraftSectionRequest struct {
	StyleProfileID string `json:"style_profile_id"`
	SessionID      string `json:"session_id"`
	PersonaID      string `json:"persona_id"`
	OutputFormatID string `json:"output_format_id"`
	DraftModel     string `json:"draft_model"`
	DraftMarkdown  string `json:"draft_markdown"`
	SectionAnchor  string `json:"section_anchor"`
}

type generateDraftResponse struct {
	Draft        string                     `json:"draft"`
	DraftPath    string                     `json:"draft_path,omitempty"`
	Evaluation   draftapp.StyleEvaluation   `json:"evaluation"`
	Verification draftapp.FinalVerification `json:"verification"`
	QualityGate  draftQualityGateDetails    `json:"quality_gate"`
}

type draftQualityGateDetails struct {
	Passed        bool                    `json:"passed"`
	Score         float64                 `json:"score"`
	FailedScore   *float64                `json:"failed_score,omitempty"`
	Runes         int                     `json:"runes"`
	Failures      []string                `json:"failures,omitempty"`
	FailedMetrics []draftFailedMetric     `json:"failed_metrics,omitempty"`
	Verification  draftVerificationStatus `json:"verification"`
	Draft         draftArtifactDetails    `json:"draft"`
}

type draftFailedMetric struct {
	Name      string   `json:"name"`
	Score     *float64 `json:"score,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`
	Missing   bool     `json:"missing,omitempty"`
	Message   string   `json:"message,omitempty"`
}

type draftVerificationStatus struct {
	Performed bool     `json:"performed"`
	Passed    bool     `json:"passed"`
	Status    string   `json:"status"`
	Summary   string   `json:"summary,omitempty"`
	Failures  []string `json:"failures,omitempty"`
}

type draftArtifactDetails struct {
	Text  string `json:"text,omitempty"`
	Path  string `json:"path,omitempty"`
	Runes int    `json:"runes"`
}

type draftGenerationErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details string `json:"details,omitempty"`
	} `json:"error"`
	Draft        string                     `json:"draft,omitempty"`
	DraftPath    string                     `json:"draft_path,omitempty"`
	Evaluation   draftapp.StyleEvaluation   `json:"evaluation,omitempty"`
	Verification draftapp.FinalVerification `json:"verification,omitempty"`
	QualityGate  *draftQualityGateDetails   `json:"quality_gate,omitempty"`
}

type regenerateDraftSectionResponse struct {
	Section              draftapp.MarkdownSection `json:"section"`
	ReplacementMarkdown  string                   `json:"replacement_markdown"`
	UpdatedDraftMarkdown string                   `json:"updated_draft_markdown"`
	RegenerationID       string                   `json:"regeneration_id,omitempty"`
}

func toGenerateDraftResponse(result draftapp.GenerateResult, draftPath string) generateDraftResponse {
	markdown := result.Draft.Markdown()
	return generateDraftResponse{
		Draft:        markdown,
		DraftPath:    draftPath,
		Evaluation:   result.Evaluation,
		Verification: result.Verification,
		QualityGate:  buildDraftQualityGateDetails(markdown, draftPath, result.Evaluation, result.Verification),
	}
}

func buildDraftQualityGateDetails(markdown, draftPath string, evaluation draftapp.StyleEvaluation, verification draftapp.FinalVerification) draftQualityGateDetails {
	score := evaluation.Comparison.Score
	var failedScore *float64
	if !evaluation.Passed {
		failedScore = &score
	}
	return draftQualityGateDetails{
		Passed:        evaluation.Passed && (!verification.Performed || verification.Passed),
		Score:         score,
		FailedScore:   failedScore,
		Runes:         len([]rune(markdown)),
		Failures:      append([]string(nil), evaluation.Failures...),
		FailedMetrics: failedMetricsFromEvaluation(evaluation),
		Verification:  verificationStatus(verification),
		Draft: draftArtifactDetails{
			Text:  markdown,
			Path:  draftPath,
			Runes: len([]rune(markdown)),
		},
	}
}

func failedMetricsFromEvaluation(evaluation draftapp.StyleEvaluation) []draftFailedMetric {
	if evaluation.Passed && len(evaluation.Failures) == 0 {
		return nil
	}
	metrics := make([]draftFailedMetric, 0, len(evaluation.Failures))
	if evaluation.Comparison.Score < evaluation.Thresholds.TotalScore {
		metrics = append(metrics, failedMetric("total_style_score", evaluation.Comparison.Score, evaluation.Thresholds.TotalScore, ""))
	}
	for _, threshold := range []struct {
		name  string
		value int
	}{
		{"paragraph_length", evaluation.Thresholds.ParagraphLength},
		{"sentence_length", evaluation.Thresholds.SentenceLength},
		{"keyword_overlap", evaluation.Thresholds.KeywordOverlap},
		{"quote_density", evaluation.Thresholds.QuoteDensity},
		{"first_person", evaluation.Thresholds.FirstPerson},
	} {
		value, ok := evaluation.Comparison.MetricScores[threshold.name]
		if !ok {
			thresholdValue := float64(threshold.value)
			metrics = append(metrics, draftFailedMetric{Name: threshold.name, Threshold: &thresholdValue, Missing: true, Message: threshold.name + " missing"})
			continue
		}
		if value < threshold.value {
			metrics = append(metrics, failedMetric(threshold.name, float64(value), float64(threshold.value), ""))
		}
	}
	for _, failure := range evaluation.Failures {
		if strings.Contains(failure, "preferred_first_person") {
			metrics = append(metrics, draftFailedMetric{Name: "preferred_first_person", Message: failure})
		}
	}
	return metrics
}

func failedMetric(name string, score, threshold float64, message string) draftFailedMetric {
	if message == "" {
		message = fmt.Sprintf("%s=%.1f below %.1f", name, score, threshold)
	}
	return draftFailedMetric{
		Name:      name,
		Score:     &score,
		Threshold: &threshold,
		Message:   message,
	}
}

func verificationStatus(verification draftapp.FinalVerification) draftVerificationStatus {
	status := "not_run"
	if verification.Performed {
		status = "failed"
		if verification.Passed {
			status = "passed"
		}
	}
	return draftVerificationStatus{
		Performed: verification.Performed,
		Passed:    verification.Passed,
		Status:    status,
		Summary:   verification.Summary,
		Failures:  append([]string(nil), verification.Failures...),
	}
}

func draftGenerationErrorPayload(result draftapp.GenerateResult, err error, code, message, draftPath string) (draftGenerationErrorResponse, bool) {
	var response draftGenerationErrorResponse
	markdown := result.Draft.Markdown()
	if strings.TrimSpace(markdown) == "" && len(result.Evaluation.Failures) == 0 && result.Evaluation.Comparison.Score == 0 && !result.Verification.Performed {
		return response, false
	}
	response.Error.Code = code
	response.Error.Message = message
	if err != nil {
		response.Error.Details = err.Error()
	}
	response.Draft = markdown
	response.DraftPath = draftPath
	response.Evaluation = result.Evaluation
	response.Verification = result.Verification
	qualityGate := buildDraftQualityGateDetails(markdown, draftPath, result.Evaluation, result.Verification)
	response.QualityGate = &qualityGate
	return response, true
}

// ListPersonasHandler returns built-in and user-authored writing personas.
func ListPersonasHandler(w http.ResponseWriter, r *http.Request) {
	personas, err := listPersonas()
	if err != nil {
		respondWithError(w, "PERSONA_LIST_FAILED", "Failed to list personas", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusOK, personas)
}

// CreatePersonaHandler stores a user-authored writing persona.
func CreatePersonaHandler(w http.ResponseWriter, r *http.Request) {
	var req createPersonaRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	persona := personadomain.Persona{
		ID:            strings.TrimSpace(req.ID),
		DisplayName:   strings.TrimSpace(req.DisplayName),
		Description:   strings.TrimSpace(req.Description),
		DefaultFormat: strings.TrimSpace(req.DefaultFormat),
		Sources:       req.Sources,
		VoiceNotes:    req.VoiceNotes,
	}
	if persona.Description == "" && persona.DisplayName != "" {
		persona.Description = "User-authored persona: " + persona.DisplayName
	}
	if strings.TrimSpace(persona.VoiceNotes.Tone) == "" && persona.DisplayName != "" {
		persona.VoiceNotes.Tone = "Write in the voice of " + persona.DisplayName + "."
	}
	if err := persona.ValidateCustom(); err != nil {
		respondWithError(w, "INVALID_PERSONA", "Invalid persona", err.Error(), http.StatusBadRequest)
		return
	}
	if _, ok := personadomain.DefaultRegistry().Get(persona.ID); ok {
		respondWithError(w, "PERSONA_ID_RESERVED", "Persona id is reserved by a built-in persona", persona.ID, http.StatusConflict)
		return
	}
	if _, ok := workflowStore.GetPersona(persona.ID); ok {
		respondWithError(w, "PERSONA_ALREADY_EXISTS", "Persona already exists", persona.ID, http.StatusConflict)
		return
	}
	if _, ok := outputformat.DefaultRegistry().Get(persona.DefaultFormat); !ok {
		respondWithError(w, "UNKNOWN_OUTPUT_FORMAT", "Output format was not found", persona.DefaultFormat, http.StatusBadRequest)
		return
	}
	if err := workflowStore.SavePersona(persona); err != nil {
		respondWithError(w, "PERSONA_SAVE_FAILED", "Failed to save persona", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusCreated, persona)
}

// UpdatePersonaHandler updates a user-authored writing persona while preserving its ID.
func UpdatePersonaHandler(w http.ResponseWriter, r *http.Request) {
	personaID := pathValue(r, "id")
	if _, ok := personadomain.DefaultRegistry().Get(personaID); ok {
		respondWithError(w, "PERSONA_ID_RESERVED", "Built-in personas cannot be updated", personaID, http.StatusConflict)
		return
	}
	persona, ok := workflowStore.GetPersona(personaID)
	if !ok {
		respondWithError(w, "PERSONA_NOT_FOUND", "Persona was not found", personaID, http.StatusNotFound)
		return
	}
	var req updatePersonaRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.ID) != "" && strings.TrimSpace(req.ID) != personaID {
		respondWithError(w, "IMMUTABLE_PERSONA_ID", "Persona id cannot be changed", req.ID, http.StatusBadRequest)
		return
	}
	if req.DisplayName != nil {
		persona.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.Description != nil {
		persona.Description = strings.TrimSpace(*req.Description)
	}
	if req.DefaultFormat != nil {
		persona.DefaultFormat = strings.TrimSpace(*req.DefaultFormat)
	}
	if req.Sources != nil {
		persona.Sources = *req.Sources
	}
	if req.VoiceNotes != nil {
		persona.VoiceNotes = *req.VoiceNotes
	}
	if persona.Description == "" && persona.DisplayName != "" {
		persona.Description = "User-authored persona: " + persona.DisplayName
	}
	if strings.TrimSpace(persona.VoiceNotes.Tone) == "" && persona.DisplayName != "" {
		persona.VoiceNotes.Tone = "Write in the voice of " + persona.DisplayName + "."
	}
	if err := persona.ValidateCustom(); err != nil {
		respondWithError(w, "INVALID_PERSONA", "Invalid persona", err.Error(), http.StatusBadRequest)
		return
	}
	if _, ok := outputformat.DefaultRegistry().Get(persona.DefaultFormat); !ok {
		respondWithError(w, "UNKNOWN_OUTPUT_FORMAT", "Output format was not found", persona.DefaultFormat, http.StatusBadRequest)
		return
	}
	if err := workflowStore.SavePersona(persona); err != nil {
		respondWithError(w, "PERSONA_SAVE_FAILED", "Failed to save persona", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusOK, persona)
}

// DeletePersonaHandler removes a custom persona only when it is not referenced by history.
func DeletePersonaHandler(w http.ResponseWriter, r *http.Request) {
	personaID := pathValue(r, "id")
	if _, ok := personadomain.DefaultRegistry().Get(personaID); ok {
		respondWithError(w, "PERSONA_ID_RESERVED", "Built-in personas cannot be deleted", personaID, http.StatusConflict)
		return
	}
	if err := workflowStore.DeletePersona(personaID); err != nil {
		switch {
		case errors.Is(err, personadomain.ErrPersonaNotFound):
			respondWithError(w, "PERSONA_NOT_FOUND", "Persona was not found", personaID, http.StatusNotFound)
		case errors.Is(err, personadomain.ErrPersonaReferenced):
			respondWithError(w, "PERSONA_REFERENCED", "Persona is referenced by workflow history", personaID, http.StatusConflict)
		default:
			respondWithError(w, "PERSONA_DELETE_FAILED", "Failed to delete persona", err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListFormatsHandler returns built-in output formats.
func ListFormatsHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, outputformat.DefaultRegistry().List())
}

// GetBriefSessionTemplateHandler returns the composed fixed-question template.
func GetBriefSessionTemplateHandler(w http.ResponseWriter, r *http.Request) {
	persona, ok := resolvePersona(r.URL.Query().Get("persona_id"))
	if !ok {
		respondWithError(w, "UNKNOWN_PERSONA", "Persona was not found", r.URL.Query().Get("persona_id"), http.StatusBadRequest)
		return
	}
	formatID := outputformat.NormalizeID(r.URL.Query().Get("format_id"))
	if strings.TrimSpace(r.URL.Query().Get("format_id")) == "" {
		formatID = persona.DefaultFormat
	}
	format, ok := outputformat.DefaultRegistry().Get(formatID)
	if !ok {
		respondWithError(w, "UNKNOWN_OUTPUT_FORMAT", "Output format was not found", formatID, http.StatusBadRequest)
		return
	}
	respondWithJSON(w, http.StatusOK, briefSessionTemplateResponse{
		PersonaID:      persona.ID,
		OutputFormatID: format.ID,
		Questions:      toArticleQuestionJSONList(briefdomain.ComposeFixedQuestions(persona.ID, format.ID)),
	})
}

// SeedAuthorStyleHandler stores a practical persona preset as a style guide.
func SeedAuthorStyleHandler(w http.ResponseWriter, r *http.Request) {
	var req seedAuthorStyleRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	persona, ok := resolvePersona(req.PersonaID)
	if !ok {
		respondWithError(w, "UNKNOWN_PERSONA", "Persona was not found", req.PersonaID, http.StatusBadRequest)
		return
	}
	formatID := req.OutputFormatID
	if strings.TrimSpace(formatID) == "" {
		formatID = persona.DefaultFormat
	}
	format, ok := outputformat.DefaultRegistry().Get(formatID)
	if !ok {
		respondWithError(w, "UNKNOWN_OUTPUT_FORMAT", "Output format was not found", formatID, http.StatusBadRequest)
		return
	}
	result, err := buildPresetAuthorStyle(persona, format)
	if err != nil {
		respondWithError(w, "AUTHOR_STYLE_PRESET_FAILED", "Failed to build persona preset", err.Error(), http.StatusInternalServerError)
		return
	}
	if err := workflowStore.SaveAuthorStyle(result); err != nil {
		respondWithError(w, "AUTHOR_STYLE_SAVE_FAILED", "Failed to save author style", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusOK, toAuthorStyleResponse(result))
}

// AnalyzeAuthorStyleHandler analyzes a writing source and stores the resulting style assets.
func AnalyzeAuthorStyleHandler(w http.ResponseWriter, r *http.Request) {
	var req analyzeAuthorStyleRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}

	persona, ok := resolvePersona(req.PersonaID)
	if !ok {
		respondWithError(w, "UNKNOWN_PERSONA", "Persona was not found", req.PersonaID, http.StatusBadRequest)
		return
	}
	formatID := req.OutputFormatID
	if strings.TrimSpace(formatID) == "" {
		formatID = persona.DefaultFormat
	}
	format, ok := outputformat.DefaultRegistry().Get(formatID)
	if !ok {
		respondWithError(w, "UNKNOWN_OUTPUT_FORMAT", "Output format was not found", formatID, http.StatusBadRequest)
		return
	}
	sourceSelector := firstNonEmpty(req.SourceSelector, req.Username, defaultStyleSourceSelector(persona, format))

	service := authorstyleapp.NewAnalyzeAuthorStyleService(sourcefetch.NewAuthorStyleFetcher(), nil)
	result, err := service.Analyze(r.Context(), authorstyleapp.AnalyzeRequest{
		Username:    sourceSelector,
		ArticleURLs: req.ArticleURLs,
		Limit:       req.Limit,
	})
	if err != nil {
		respondWithError(w, "AUTHOR_STYLE_ANALYSIS_FAILED", "Failed to analyze author style", err.Error(), http.StatusInternalServerError)
		return
	}
	if err := workflowStore.SaveAuthorStyle(result); err != nil {
		respondWithError(w, "AUTHOR_STYLE_SAVE_FAILED", "Failed to save author style", err.Error(), http.StatusInternalServerError)
		return
	}
	if strings.TrimSpace(req.StyleModel) != "" {
		if refined, err := refineStyleGuideWithModel(r.Context(), result, req.StyleModel); err == nil {
			result.Guide.Markdown = refined
			_ = workflowStore.SaveAuthorStyle(result)
		}
	}

	respondWithJSON(w, http.StatusOK, toAuthorStyleResponse(result))
}

func refineStyleGuideWithModel(ctx context.Context, result authorstyleapp.AnalyzeResult, model string) (string, error) {
	generator, err := llamacpp.NewClientFromEnvForPurposeWithModel("STYLE", model)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	var titles []string
	for _, article := range result.Source.Articles {
		if strings.TrimSpace(article.Title) != "" {
			titles = append(titles, article.Title)
		}
	}
	prompt := fmt.Sprintf(`あなたはNote記事の文体分析者です。
以下の機械的な文体ガイドと記事タイトル群をもとに、後続の下書き生成で使いやすい日本語Markdownの箇条書きに整えてください。
事実を捏造せず、文体、頻出テーマ、導入、締め方、避けるべき癖だけを簡潔にまとめてください。
出力はMarkdown箇条書きのみ。

記事タイトル:
%s

機械的な文体ガイド:
%s
`, strings.Join(titles, "\n"), result.Guide.Markdown)
	return generator.Generate(ctx, prompt)
}

func defaultStyleSourceSelector(persona personadomain.Persona, format outputformat.OutputFormat) string {
	source := defaultStyleSource(persona, format)
	if strings.TrimSpace(source.Kind) == "" {
		return ""
	}
	if strings.TrimSpace(source.Ref) != "" {
		return source.Kind + ":" + source.Ref
	}
	if strings.TrimSpace(source.URL) != "" {
		return source.Kind + ":" + source.URL
	}
	return ""
}

func defaultStyleSource(persona personadomain.Persona, format outputformat.OutputFormat) personadomain.AuthorSource {
	switch format.ID {
	case outputformat.IDMarkdownBlog, outputformat.IDHomepageSection:
		if source, ok := findPersonaSource(persona, "github"); ok {
			return source
		}
		if source, ok := findPersonaSource(persona, "rss"); ok {
			return source
		}
	case outputformat.IDZennArticle:
		if source, ok := findPersonaSource(persona, "zenn"); ok {
			return source
		}
	case outputformat.IDQiitaArticle:
		if source, ok := findPersonaSource(persona, "qiita"); ok {
			return source
		}
	case outputformat.IDNoteArticle:
		if source, ok := findPersonaSource(persona, "note"); ok {
			return source
		}
	}
	if len(persona.Sources) > 0 {
		return persona.Sources[0]
	}
	return personadomain.AuthorSource{}
}

func findPersonaSource(persona personadomain.Persona, kind string) (personadomain.AuthorSource, bool) {
	for _, source := range persona.Sources {
		if strings.EqualFold(strings.TrimSpace(source.Kind), kind) {
			return source, true
		}
	}
	return personadomain.AuthorSource{}, false
}

func resolvePersona(id string) (personadomain.Persona, bool) {
	normalized := personadomain.NormalizeID(id)
	if persona, ok := personadomain.DefaultRegistry().Get(normalized); ok {
		return persona, true
	}
	return workflowStore.GetPersona(normalized)
}

func listPersonas() ([]personadomain.Persona, error) {
	builtIns := personadomain.DefaultRegistry().List()
	custom, err := workflowStore.ListPersonas()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(custom, func(i, j int) bool {
		return custom[i].ID < custom[j].ID
	})
	personas := make([]personadomain.Persona, 0, len(builtIns)+len(custom))
	personas = append(personas, builtIns...)
	seen := map[string]bool{}
	for _, persona := range builtIns {
		seen[persona.ID] = true
	}
	for _, persona := range custom {
		if !seen[persona.ID] {
			personas = append(personas, persona)
		}
	}
	return personas, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if cleaned := strings.TrimSpace(value); cleaned != "" {
			return cleaned
		}
	}
	return ""
}

func formatSpecificPresetNote(formatID string) string {
	switch formatID {
	case outputformat.IDMarkdownBlog:
		return "自社ブログでは、技術的な知見の報告、実装判断、検証結果、社員へのビジョン共有を中心に、断定口調で具体的に書く。"
	case outputformat.IDZennArticle:
		return "Zennでは、技術の前提、手順、コード、つまずき、検証結果を開発者向けに整理して書く。"
	case outputformat.IDQiitaArticle:
		return "Qiitaでは、再現手順、環境、コード例、結果、参考リンクを実用重視で書く。"
	case outputformat.IDHomepageSection:
		return "ホームページでは、会社としての信頼、価値提案、次の行動がすぐ伝わる短いHTMLセクションとして書く。"
	default:
		return "noteでは、体験、違和感、技術的な試み、読者への提案を読み物として自然につなぐ。"
	}
}

func buildPresetAuthorStyle(persona personadomain.Persona, format outputformat.OutputFormat) (authorstyleapp.AnalyzeResult, error) {
	fetchedAt := time.Now().UTC()
	content := strings.Join([]string{
		persona.Description,
		persona.VoiceNotes.Tone,
		format.DisplayName + ": " + format.Description,
		format.PromptFragment,
		formatSpecificPresetNote(format.ID),
		strings.Join(persona.VoiceNotes.FirstPerson, " "),
		strings.Join(persona.VoiceNotes.TitlePatterns, " "),
		strings.Repeat(" "+strings.Join(persona.VoiceNotes.AntiPatterns, " "), 2),
	}, "\n\n")
	if strings.TrimSpace(content) == "" {
		content = persona.DisplayName + " writing preset"
	}
	articleURL := "preset://" + persona.ID + "/" + format.ID
	if source := defaultStyleSource(persona, format); strings.TrimSpace(source.URL) != "" {
		articleURL = source.URL
	}
	article := articledomain.Article{
		URL:     articleURL,
		Title:   persona.DisplayName + " / " + format.DisplayName + " preset",
		Content: strings.Repeat(content+"\n\n", 8),
	}
	source := authordomain.AuthorSource{
		Username: persona.ID,
		Articles: []authordomain.SourceArticle{{
			ID:    persona.ID + "_preset",
			URL:   articleURL,
			Title: article.Title,
			At:    fetchedAt,
		}},
		FetchedAt: fetchedAt,
	}
	profile, err := authordomain.BuildAuthorStyleProfile(source, []articledomain.Article{article})
	if err != nil {
		return authorstyleapp.AnalyzeResult{}, err
	}
	if len(persona.VoiceNotes.FirstPerson) > 0 {
		profile.PreferredFirstPerson = persona.VoiceNotes.FirstPerson[0]
	}
	guide, err := authordomain.BuildWritingStyleGuide(profile)
	if err != nil {
		return authorstyleapp.AnalyzeResult{}, err
	}
	if len(persona.VoiceNotes.FirstPerson) > 0 {
		guide.PreferredFirstPerson = persona.VoiceNotes.FirstPerson[0]
	}
	guide.RecurringThemes = append([]string(nil), persona.VoiceNotes.TitlePatterns...)
	if len(guide.RecurringThemes) == 0 {
		guide.RecurringThemes = []string{persona.DisplayName, "技術", "体験"}
	}
	guide.ParagraphRhythm = persona.VoiceNotes.Tone + "\n" + formatSpecificPresetNote(format.ID)
	guide.HeadingGuidance = "出力先「" + format.DisplayName + "」の形式に合わせ、読者が流れを追いやすい見出しを置く"
	guide.OpeningPatterns = append([]string(nil), persona.VoiceNotes.TitlePatterns...)
	guide.ConclusionPatterns = []string{"読者が次に試せる具体的な一歩で締める"}
	guide.Warnings = append([]string{"persona_preset_without_live_fetch"}, persona.VoiceNotes.AntiPatterns...)
	guide.Markdown = authordomain.GuideMarkdown(guide)
	if err := guide.Validate(); err != nil {
		return authorstyleapp.AnalyzeResult{}, err
	}
	return authorstyleapp.AnalyzeResult{
		ID:           authorstyleapp.ResultID(source, profile.ID, guide.ID),
		Source:       source,
		Profile:      profile,
		Guide:        guide,
		ArticleCount: 1,
		CreatedAt:    fetchedAt,
	}, nil
}

// ListAuthorStylesHandler returns stored style-guide artifacts for picker UIs.
func ListAuthorStylesHandler(w http.ResponseWriter, r *http.Request) {
	results, err := workflowStore.ListAuthorStyles()
	if err != nil {
		respondWithError(w, "AUTHOR_STYLE_LIST_FAILED", "Failed to list author styles", err.Error(), http.StatusInternalServerError)
		return
	}
	sortAuthorStyles(results)
	respondWithJSON(w, http.StatusOK, authorStyleListResponse{
		StyleGuides: toStyleGuideArtifactResponses(results),
	})
}

// GetAuthorStyleHandler returns a stored author style analysis result.
func GetAuthorStyleHandler(w http.ResponseWriter, r *http.Request) {
	id := pathValue(r, "id")
	result, ok := workflowStore.GetAuthorStyle(id)
	if !ok {
		respondWithError(w, "AUTHOR_STYLE_NOT_FOUND", "Author style was not found", "", http.StatusNotFound)
		return
	}
	respondWithJSON(w, http.StatusOK, toAuthorStyleResponse(result))
}

// CreateStyleGuideVersionHandler stores an edited style guide as a new version.
func CreateStyleGuideVersionHandler(w http.ResponseWriter, r *http.Request) {
	var req updateStyleGuideRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	base, ok := workflowStore.GetAuthorStyle(pathValue(r, "id"))
	if !ok {
		respondWithError(w, "AUTHOR_STYLE_NOT_FOUND", "Author style was not found", "", http.StatusNotFound)
		return
	}
	updated, err := styleGuideVersionFromRequest(base, req)
	if err != nil {
		respondWithError(w, "INVALID_STYLE_GUIDE_VERSION", "Invalid style guide version", err.Error(), http.StatusBadRequest)
		return
	}
	if err := workflowStore.SaveAuthorStyle(updated); err != nil {
		respondWithError(w, "AUTHOR_STYLE_SAVE_FAILED", "Failed to save author style", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusCreated, toStyleGuideArtifactResponse(updated))
}

// ListBriefSessionsHandler returns saved interview sessions for project history UIs.
func ListBriefSessionsHandler(w http.ResponseWriter, r *http.Request) {
	sessions, err := workflowStore.ListSessions()
	if err != nil {
		respondWithError(w, "BRIEF_SESSION_LIST_FAILED", "Failed to list brief sessions", err.Error(), http.StatusInternalServerError)
		return
	}
	briefs, err := workflowStore.ListBriefs()
	if err != nil {
		respondWithError(w, "BRIEF_LIST_FAILED", "Failed to list completed briefs", err.Error(), http.StatusInternalServerError)
		return
	}
	sortBriefSessions(sessions)
	respondWithJSON(w, http.StatusOK, briefSessionListResponse{
		Sessions: toBriefSessionSummaryResponses(sessions, briefs),
	})
}

// CreateBriefSessionHandler starts the fixed-question interview.
func CreateBriefSessionHandler(w http.ResponseWriter, r *http.Request) {
	var req createBriefSessionRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.StyleProfileID) == "" {
		respondWithError(w, "MISSING_REQUIRED_FIELD", "style_profile_id is required", "", http.StatusBadRequest)
		return
	}
	if _, _, ok := workflowStore.GetProfileAndGuide(req.StyleProfileID); !ok {
		respondWithError(w, "AUTHOR_STYLE_NOT_FOUND", "Author style was not found", "", http.StatusNotFound)
		return
	}
	if strings.TrimSpace(req.SessionID) == "" {
		req.SessionID = newID("abs")
	}
	persona, ok := resolvePersona(req.PersonaID)
	if !ok {
		respondWithError(w, "UNKNOWN_PERSONA", "Persona was not found", req.PersonaID, http.StatusBadRequest)
		return
	}
	formatID := outputformat.NormalizeID(req.OutputFormatID)
	if strings.TrimSpace(req.OutputFormatID) == "" {
		formatID = persona.DefaultFormat
	}
	if _, ok := outputformat.DefaultRegistry().Get(formatID); !ok {
		respondWithError(w, "UNKNOWN_OUTPUT_FORMAT", "Output format was not found", formatID, http.StatusBadRequest)
		return
	}

	service := newBriefInterviewService(req.BriefModel)
	result, err := service.StartSession(briefapp.StartSessionInput{
		SessionID:      req.SessionID,
		StyleProfileID: req.StyleProfileID,
		PersonaID:      persona.ID,
		OutputFormatID: formatID,
		Questions:      toDomainQuestions(req.Questions),
	})
	if err != nil {
		respondWithError(w, "BRIEF_SESSION_CREATE_FAILED", "Failed to create brief session", err.Error(), http.StatusBadRequest)
		return
	}
	if err := workflowStore.SaveSession(result.Session); err != nil {
		respondWithError(w, "BRIEF_SESSION_SAVE_FAILED", "Failed to save brief session", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusOK, toBriefSessionResponse(result))
}

// GetBriefSessionHandler returns the current interview state.
func GetBriefSessionHandler(w http.ResponseWriter, r *http.Request) {
	session, ok := workflowStore.GetSession(pathValue(r, "id"))
	if !ok {
		respondWithError(w, "BRIEF_SESSION_NOT_FOUND", "Brief session was not found", "", http.StatusNotFound)
		return
	}
	result := briefapp.InterviewResult{Session: session, Completed: session.Completed}
	if session.Completed {
		brief := session.AssembleBrief()
		result.Brief = &brief
	} else if question, ok := session.CurrentQuestion(); ok {
		result.NextQuestion = &question
	}
	respondWithJSON(w, http.StatusOK, toBriefSessionResponse(result))
}

// ListBriefArtifactsHandler returns completed brief artifacts for reuse.
func ListBriefArtifactsHandler(w http.ResponseWriter, r *http.Request) {
	briefs, err := workflowStore.ListBriefs()
	if err != nil {
		respondWithError(w, "BRIEF_LIST_FAILED", "Failed to list completed briefs", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusOK, briefArtifactListResponse{
		Briefs: listBriefArtifactResponses(briefs),
	})
}

// GetBriefArtifactHandler returns one completed brief artifact by session ID.
func GetBriefArtifactHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := pathValue(r, "id")
	articleBrief, ok := workflowStore.GetBrief(sessionID)
	if !ok {
		respondWithError(w, "BRIEF_NOT_FOUND", "Brief was not found", sessionID, http.StatusNotFound)
		return
	}
	session, sessionOK := workflowStore.GetSession(sessionID)
	respondWithJSON(w, http.StatusOK, toBriefArtifactResponse(sessionID, articleBrief, session, sessionOK))
}

// ListBriefVersionsHandler returns persisted versions for one completed brief artifact.
func ListBriefVersionsHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := pathValue(r, "id")
	if _, ok := workflowStore.GetBrief(sessionID); !ok {
		respondWithError(w, "BRIEF_NOT_FOUND", "Brief was not found", sessionID, http.StatusNotFound)
		return
	}
	versions, err := workflowStore.ListBriefVersions(sessionID)
	if err != nil {
		respondWithError(w, "BRIEF_VERSION_LIST_FAILED", "Failed to list brief versions", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusOK, briefVersionListResponse{
		SessionID:      sessionID,
		CurrentVersion: currentBriefVersion(versions),
		Versions:       toBriefVersionResponses(versions),
	})
}

// UpdateBriefArtifactHandler updates the saved brief artifact without rewriting session answers.
func UpdateBriefArtifactHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := pathValue(r, "id")
	articleBrief, ok := workflowStore.GetBrief(sessionID)
	if !ok {
		respondWithError(w, "BRIEF_NOT_FOUND", "Brief was not found", sessionID, http.StatusNotFound)
		return
	}
	fields, err := decodeBriefUpdateFields(r)
	if err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", err.Error(), http.StatusBadRequest)
		return
	}
	if err := applyBriefUpdateFields(&articleBrief, fields); err != nil {
		respondWithError(w, "INVALID_BRIEF_UPDATE", "Invalid brief update", err.Error(), http.StatusBadRequest)
		return
	}
	if err := workflowStore.SaveBrief(sessionID, articleBrief); err != nil {
		respondWithError(w, "BRIEF_SAVE_FAILED", "Failed to save article brief", err.Error(), http.StatusInternalServerError)
		return
	}
	session, sessionOK := workflowStore.GetSession(sessionID)
	respondWithJSON(w, http.StatusOK, updateBriefResponse{
		Brief: toBriefArtifactResponse(sessionID, articleBrief, session, sessionOK),
	})
}

// ListWorkflowArtifactsHandler returns all currently reusable workflow artifacts.
func ListWorkflowArtifactsHandler(w http.ResponseWriter, r *http.Request) {
	styles, err := workflowStore.ListAuthorStyles()
	if err != nil {
		respondWithError(w, "AUTHOR_STYLE_LIST_FAILED", "Failed to list author styles", err.Error(), http.StatusInternalServerError)
		return
	}
	briefs, err := workflowStore.ListBriefs()
	if err != nil {
		respondWithError(w, "BRIEF_LIST_FAILED", "Failed to list completed briefs", err.Error(), http.StatusInternalServerError)
		return
	}
	sessions, err := workflowStore.ListSessions()
	if err != nil {
		respondWithError(w, "BRIEF_SESSION_LIST_FAILED", "Failed to list brief sessions", err.Error(), http.StatusInternalServerError)
		return
	}
	sortAuthorStyles(styles)
	sortBriefSessions(sessions)
	response := workflowArtifactsResponse{
		StyleGuides: toStyleGuideArtifactResponses(styles),
		Sessions:    toBriefSessionSummaryResponses(sessions, briefs),
		Briefs:      listBriefArtifactResponses(briefs),
	}
	if store, ok := workflowStore.(workflowHistoryReader); ok {
		if projects, articles, drafts, err := listWorkflowHistoryResponses(store); err != nil {
			respondWithError(w, "WORKFLOW_HISTORY_LIST_FAILED", "Failed to list project history", err.Error(), http.StatusInternalServerError)
			return
		} else {
			response.Projects = projects
			response.Articles = articles
			response.Drafts = drafts
		}
	}
	respondWithJSON(w, http.StatusOK, response)
}

// ListProjectsHandler returns SQLite-backed project history when available.
func ListProjectsHandler(w http.ResponseWriter, r *http.Request) {
	store, ok := workflowStore.(workflowHistoryReader)
	if !ok {
		respondWithJSON(w, http.StatusOK, projectHistoryListResponse{Projects: []projectHistoryResponse{}})
		return
	}
	projects, err := store.ListProjects()
	if err != nil {
		respondWithError(w, "PROJECT_LIST_FAILED", "Failed to list projects", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusOK, projectHistoryListResponse{Projects: toProjectHistoryResponses(projects)})
}

// GetProjectHandler returns one project with its article and source history.
func GetProjectHandler(w http.ResponseWriter, r *http.Request) {
	store, ok := workflowStore.(workflowHistoryReader)
	if !ok {
		respondWithError(w, "PROJECT_NOT_FOUND", "Project was not found", "", http.StatusNotFound)
		return
	}
	projectID := pathValue(r, "id")
	project, ok := store.GetProject(projectID)
	if !ok {
		respondWithError(w, "PROJECT_NOT_FOUND", "Project was not found", projectID, http.StatusNotFound)
		return
	}
	articles, err := store.ListArticlesByProject(projectID)
	if err != nil {
		respondWithError(w, "PROJECT_ARTICLES_LIST_FAILED", "Failed to list project articles", err.Error(), http.StatusInternalServerError)
		return
	}
	sourceSnapshots, err := store.ListSourceSnapshots("project", projectID)
	if err != nil {
		respondWithError(w, "PROJECT_SOURCE_SNAPSHOT_LIST_FAILED", "Failed to list project source snapshots", err.Error(), http.StatusInternalServerError)
		return
	}
	articleResponses := make([]articleHistoryResponse, 0, len(articles))
	for _, article := range articles {
		response, _, err := articleHistoryResponseWithDetails(store, article)
		if err != nil {
			respondWithError(w, "PROJECT_ARTICLE_HISTORY_FAILED", "Failed to load project article history", err.Error(), http.StatusInternalServerError)
			return
		}
		articleResponses = append(articleResponses, response)
	}
	projectResponse := toProjectHistoryResponse(project)
	projectResponse.Articles = articleResponses
	projectResponse.SourceSnapshots = toSourceSnapshotHistoryResponses(sourceSnapshots)
	respondWithJSON(w, http.StatusOK, projectHistoryDetailResponse{
		Project:         projectResponse,
		Articles:        articleResponses,
		SourceSnapshots: projectResponse.SourceSnapshots,
	})
}

// GetArticleHandler returns one article with draft versions and source history.
func GetArticleHandler(w http.ResponseWriter, r *http.Request) {
	store, ok := workflowStore.(workflowHistoryReader)
	if !ok {
		respondWithError(w, "ARTICLE_NOT_FOUND", "Article was not found", "", http.StatusNotFound)
		return
	}
	articleID := pathValue(r, "id")
	article, ok := store.GetArticle(articleID)
	if !ok {
		respondWithError(w, "ARTICLE_NOT_FOUND", "Article was not found", articleID, http.StatusNotFound)
		return
	}
	articleResponse, _, err := articleHistoryResponseWithDetails(store, article)
	if err != nil {
		respondWithError(w, "ARTICLE_HISTORY_FAILED", "Failed to load article history", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusOK, articleHistoryDetailResponse{
		Article:         articleResponse,
		Drafts:          articleResponse.Drafts,
		SourceSnapshots: articleResponse.SourceSnapshots,
	})
}

// GetDraftHandler returns one draft with regeneration history.
func GetDraftHandler(w http.ResponseWriter, r *http.Request) {
	store, ok := workflowStore.(workflowHistoryReader)
	if !ok {
		respondWithError(w, "DRAFT_NOT_FOUND", "Draft was not found", "", http.StatusNotFound)
		return
	}
	draftID := pathValue(r, "id")
	draft, ok := store.GetDraft(draftID)
	if !ok {
		respondWithError(w, "DRAFT_NOT_FOUND", "Draft was not found", draftID, http.StatusNotFound)
		return
	}
	draftResponse, err := draftHistoryResponseWithDetails(store, draft)
	if err != nil {
		respondWithError(w, "DRAFT_HISTORY_FAILED", "Failed to load draft history", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusOK, draftHistoryDetailResponse{
		Draft:                draftResponse,
		SourceSnapshots:      draftResponse.SourceSnapshots,
		SectionRegenerations: draftResponse.SectionRegenerations,
	})
}

// EditBriefAnswerHandler creates a new child session from an edited past answer.
func EditBriefAnswerHandler(w http.ResponseWriter, r *http.Request) {
	var req editBriefAnswerRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	session, ok := workflowStore.GetSession(pathValue(r, "id"))
	if !ok {
		respondWithError(w, "BRIEF_SESSION_NOT_FOUND", "Brief session was not found", "", http.StatusNotFound)
		return
	}
	service := newBriefInterviewServiceForSession(session, req.BriefModel)
	result, err := service.ForkAnswer(r.Context(), session, newID("abs"), pathValue(r, "answer_id"), req.Content)
	if err != nil {
		respondWithError(w, "BRIEF_ANSWER_EDIT_FAILED", "Failed to edit brief answer", err.Error(), http.StatusBadRequest)
		return
	}
	if err := workflowStore.SaveSession(result.Session); err != nil {
		respondWithError(w, "BRIEF_SESSION_SAVE_FAILED", "Failed to save brief session", err.Error(), http.StatusInternalServerError)
		return
	}
	if result.Completed && result.Brief != nil {
		if err := workflowStore.SaveBrief(result.Session.ID, *result.Brief); err != nil {
			respondWithError(w, "BRIEF_SAVE_FAILED", "Failed to save article brief", err.Error(), http.StatusInternalServerError)
			return
		}
	}
	respondWithJSON(w, http.StatusOK, toBriefSessionResponse(result))
}

// AnswerBriefSessionHandler records one interview answer and returns the next question.
func AnswerBriefSessionHandler(w http.ResponseWriter, r *http.Request) {
	var req answerBriefSessionRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	session, ok := workflowStore.GetSession(pathValue(r, "id"))
	if !ok {
		respondWithError(w, "BRIEF_SESSION_NOT_FOUND", "Brief session was not found", "", http.StatusNotFound)
		return
	}
	if wantsEventStream(r) && !req.SkipDeepDive {
		streamAnswerBriefSession(w, r, req, session)
		return
	}

	var result briefapp.InterviewResult
	if req.SkipDeepDive {
		session.MarkDeepDiveSkipped()
		brief, err := session.Complete()
		if err != nil {
			respondWithError(w, "BRIEF_SESSION_INCOMPLETE", "Brief session is not complete", err.Error(), http.StatusBadRequest)
			return
		}
		result = briefapp.InterviewResult{Session: session, Brief: &brief, Completed: true}
	} else {
		service := newBriefInterviewServiceForSession(session, req.BriefModel)
		next, err := service.Answer(r.Context(), session, req.Content)
		if err != nil {
			respondWithError(w, "BRIEF_ANSWER_FAILED", "Failed to record brief answer", err.Error(), http.StatusBadRequest)
			return
		}
		result = next
	}
	if err := workflowStore.SaveSession(result.Session); err != nil {
		respondWithError(w, "BRIEF_SESSION_SAVE_FAILED", "Failed to save brief session", err.Error(), http.StatusInternalServerError)
		return
	}
	if result.Completed && result.Brief != nil {
		if err := workflowStore.SaveBrief(result.Session.ID, *result.Brief); err != nil {
			respondWithError(w, "BRIEF_SAVE_FAILED", "Failed to save article brief", err.Error(), http.StatusInternalServerError)
			return
		}
	}
	respondWithJSON(w, http.StatusOK, toBriefSessionResponse(result))
}

func streamAnswerBriefSession(w http.ResponseWriter, r *http.Request, req answerBriefSessionRequest, session briefdomain.ArticleBriefSession) {
	stream, ok := newSSEStream(w)
	if !ok {
		respondWithError(w, "STREAMING_UNSUPPORTED", "Streaming is not supported by this response writer", "", http.StatusInternalServerError)
		return
	}
	service := newBriefInterviewServiceForSession(session, req.BriefModel)
	model := strings.TrimSpace(req.BriefModel)
	stopHeartbeat := stream.StartHeartbeat(r.Context(), "follow_up", "", model, 10*time.Second)
	defer stopHeartbeat()
	result, err := service.AnswerStream(r.Context(), session, req.Content, briefapp.StreamEvents{
		OnStatus: func(status string) error {
			return stream.Send("status", streamStatus{Status: status, Phase: "follow_up", Model: model, StartedAt: stream.started.Format(time.RFC3339), ElapsedMS: stream.ElapsedMS()})
		},
		OnFollowUpChunk: func(chunk string) error {
			return stream.Send("chunk", streamChunk{Text: chunk, ElapsedMS: stream.ElapsedMS()})
		},
	})
	if err != nil {
		_ = stream.Send("error", streamError{Code: "BRIEF_ANSWER_FAILED", Message: "Failed to record brief answer", Detail: err.Error(), ElapsedMS: stream.ElapsedMS()})
		return
	}
	if err := workflowStore.SaveSession(result.Session); err != nil {
		_ = stream.Send("error", streamError{Code: "BRIEF_SESSION_SAVE_FAILED", Message: "Failed to save brief session", Detail: err.Error(), ElapsedMS: stream.ElapsedMS()})
		return
	}
	if result.Completed && result.Brief != nil {
		if err := workflowStore.SaveBrief(result.Session.ID, *result.Brief); err != nil {
			_ = stream.Send("error", streamError{Code: "BRIEF_SAVE_FAILED", Message: "Failed to save article brief", Detail: err.Error(), ElapsedMS: stream.ElapsedMS()})
			return
		}
	}
	_ = stream.Send("result", toBriefSessionResponse(result))
	_ = stream.Send("done", streamStatus{Status: "completed", ElapsedMS: stream.ElapsedMS()})
}

// GenerateDraftHandler generates a draft from a stored style guide and completed brief.
func GenerateDraftHandler(w http.ResponseWriter, r *http.Request) {
	var req generateDraftRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	profile, guide, ok := workflowStore.GetProfileAndGuide(req.StyleProfileID)
	if !ok {
		respondWithError(w, "AUTHOR_STYLE_NOT_FOUND", "Author style was not found", "", http.StatusNotFound)
		return
	}
	articleBrief, ok := workflowStore.GetBrief(req.SessionID)
	if !ok {
		session, sessionOK := workflowStore.GetSession(req.SessionID)
		if !sessionOK || !session.Completed {
			respondWithError(w, "BRIEF_NOT_FOUND", "Completed article brief was not found", "", http.StatusBadRequest)
			return
		}
		articleBrief = session.AssembleBrief()
	}
	personaID := strings.TrimSpace(req.PersonaID)
	formatID := strings.TrimSpace(req.OutputFormatID)
	if personaID == "" {
		personaID = articleBrief.PersonaID
	}
	if formatID == "" {
		formatID = articleBrief.OutputFormatID
	}
	persona, ok := resolvePersona(personaID)
	if !ok {
		respondWithError(w, "UNKNOWN_PERSONA", "Persona was not found", personaID, http.StatusBadRequest)
		return
	}
	if formatID == "" {
		formatID = persona.DefaultFormat
	}
	format, ok := outputformat.DefaultRegistry().Get(formatID)
	if !ok {
		respondWithError(w, "UNKNOWN_OUTPUT_FORMAT", "Output format was not found", formatID, http.StatusBadRequest)
		return
	}
	articleBrief.PersonaID = persona.ID
	articleBrief.OutputFormatID = format.ID

	if wantsEventStream(r) {
		streamGenerateDraft(w, r, req, profile, guide, articleBrief, persona, format)
		return
	}

	generator, err := llamacpp.NewClientFromEnvForPurposeWithModel("DRAFT", req.DraftModel)
	if err != nil {
		respondWithError(w, "GENERATOR_INITIALIZATION_FAILED", "Failed to initialize local LLM client", err.Error(), http.StatusInternalServerError)
		return
	}
	service, err := newDraftServiceWithVerifier(generator, req.VerifyModel)
	if err != nil {
		respondWithError(w, "VERIFIER_INITIALIZATION_FAILED", "Failed to initialize verification LLM client", err.Error(), http.StatusInternalServerError)
		return
	}
	result, err := service.Generate(r.Context(), draftapp.GenerateRequest{
		StyleGuide:    guide,
		Brief:         articleBrief,
		AuthorProfile: profile,
		Persona:       persona,
		OutputFormat:  format,
	})
	if err != nil {
		if response, ok := draftGenerationErrorPayload(result, err, "DRAFT_GENERATION_FAILED", "Failed to generate draft", ""); ok {
			respondWithJSON(w, http.StatusUnprocessableEntity, response)
			return
		}
		respondWithError(w, "DRAFT_GENERATION_FAILED", "Failed to generate draft", err.Error(), http.StatusInternalServerError)
		return
	}
	draftPath := saveGeneratedDraftHistory(req, result, articleBrief, persona, format)
	respondWithJSON(w, http.StatusOK, toGenerateDraftResponse(result, draftPath))
}

func streamGenerateDraft(w http.ResponseWriter, r *http.Request, req generateDraftRequest, profile authordomain.AuthorStyleProfile, guide authordomain.WritingStyleGuide, articleBrief briefdomain.ArticleBrief, persona personadomain.Persona, format outputformat.OutputFormat) {
	stream, ok := newSSEStream(w)
	if !ok {
		respondWithError(w, "STREAMING_UNSUPPORTED", "Streaming is not supported by this response writer", "", http.StatusInternalServerError)
		return
	}
	generator, err := llamacpp.NewClientFromEnvForPurposeWithModel("DRAFT", req.DraftModel)
	if err != nil {
		_ = stream.Send("error", streamError{Code: "GENERATOR_INITIALIZATION_FAILED", Message: "Failed to initialize local LLM client", Detail: err.Error(), ElapsedMS: stream.ElapsedMS()})
		return
	}
	endpoint := generator.BaseURL()
	model := generator.Model()
	_ = stream.Send("status", streamStatus{Status: "runtime_connected", Phase: "draft", Endpoint: endpoint, Model: model, StartedAt: stream.started.Format(time.RFC3339), ElapsedMS: stream.ElapsedMS()})
	stopHeartbeat := stream.StartHeartbeat(r.Context(), "draft", endpoint, model, 10*time.Second)
	defer stopHeartbeat()
	service, err := newDraftServiceWithVerifier(generator, req.VerifyModel)
	if err != nil {
		_ = stream.Send("error", streamError{Code: "VERIFIER_INITIALIZATION_FAILED", Message: "Failed to initialize verification LLM client", Detail: err.Error(), ElapsedMS: stream.ElapsedMS()})
		return
	}
	result, err := service.GenerateStream(r.Context(), draftapp.GenerateRequest{
		StyleGuide:    guide,
		Brief:         articleBrief,
		AuthorProfile: profile,
		Persona:       persona,
		OutputFormat:  format,
	}, draftapp.StreamEvents{
		OnStatus: func(status string) error {
			return stream.Send("status", streamStatus{Status: status, Phase: "draft", Endpoint: endpoint, Model: model, StartedAt: stream.started.Format(time.RFC3339), ElapsedMS: stream.ElapsedMS()})
		},
		OnChunk: func(chunk string) error {
			return stream.Send("chunk", streamChunk{Text: chunk, ElapsedMS: stream.ElapsedMS()})
		},
	})
	if err != nil {
		if response, ok := draftGenerationErrorPayload(result, err, "DRAFT_GENERATION_FAILED", "Failed to generate draft", ""); ok {
			_ = stream.Send("error", streamError{
				Code:         response.Error.Code,
				Message:      response.Error.Message,
				Detail:       response.Error.Details,
				ElapsedMS:    stream.ElapsedMS(),
				Draft:        response.Draft,
				DraftPath:    response.DraftPath,
				Evaluation:   &response.Evaluation,
				Verification: &response.Verification,
				QualityGate:  response.QualityGate,
			})
			return
		}
		_ = stream.Send("error", streamError{Code: "DRAFT_GENERATION_FAILED", Message: "Failed to generate draft", Detail: err.Error(), ElapsedMS: stream.ElapsedMS()})
		return
	}
	draftPath := saveGeneratedDraftHistory(req, result, articleBrief, persona, format)
	_ = stream.Send("result", toGenerateDraftResponse(result, draftPath))
	_ = stream.Send("done", streamStatus{Status: "completed", Phase: "draft", Endpoint: endpoint, Model: model, StartedAt: stream.started.Format(time.RFC3339), ElapsedMS: stream.ElapsedMS(), Runes: len([]rune(result.Draft.Markdown())), Score: result.Evaluation.Comparison.Score})
}

// RegenerateDraftSectionHandler rewrites only one h2 subtree of the current draft.
func RegenerateDraftSectionHandler(w http.ResponseWriter, r *http.Request) {
	var req regenerateDraftSectionRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	pathID := pathValue(r, "id")
	baseDraft, hasBaseDraft := historyDraftFromPath(pathID)
	if hasBaseDraft {
		if strings.TrimSpace(req.SessionID) == "" {
			req.SessionID = baseDraft.SessionID
		}
		if strings.TrimSpace(req.StyleProfileID) == "" {
			req.StyleProfileID = baseDraft.StyleProfileID
		}
		if strings.TrimSpace(req.PersonaID) == "" {
			req.PersonaID = baseDraft.PersonaID
		}
		if strings.TrimSpace(req.OutputFormatID) == "" {
			req.OutputFormatID = baseDraft.OutputFormatID
		}
		if strings.TrimSpace(req.DraftMarkdown) == "" {
			req.DraftMarkdown = baseDraft.Markdown
		}
	}
	if strings.TrimSpace(req.SessionID) == "" {
		req.SessionID = pathID
	}
	profile, guide, articleBrief, persona, format, ok := draftContextFromRequest(req.StyleProfileID, req.SessionID, req.PersonaID, req.OutputFormatID)
	if !ok {
		respondWithError(w, "DRAFT_CONTEXT_NOT_FOUND", "Draft context was not found", "", http.StatusBadRequest)
		return
	}
	generator, err := llamacpp.NewClientFromEnvForPurposeWithModel("DRAFT", req.DraftModel)
	if err != nil {
		respondWithError(w, "GENERATOR_INITIALIZATION_FAILED", "Failed to initialize local LLM client", err.Error(), http.StatusInternalServerError)
		return
	}
	service := draftapp.NewService(generator)
	result, err := service.RegenerateSection(r.Context(), draftapp.RegenerateSectionRequest{
		GenerateRequest: draftapp.GenerateRequest{
			StyleGuide:    guide,
			Brief:         articleBrief,
			AuthorProfile: profile,
			Persona:       persona,
			OutputFormat:  format,
		},
		DraftMarkdown: req.DraftMarkdown,
		SectionAnchor: req.SectionAnchor,
	})
	if err != nil {
		respondWithError(w, "DRAFT_SECTION_REGENERATE_FAILED", "Failed to regenerate draft section", err.Error(), http.StatusBadRequest)
		return
	}
	regenerationID := saveSectionRegenerationHistory(baseDraft, hasBaseDraft, req, result)
	respondWithJSON(w, http.StatusOK, regenerateDraftSectionResponse{
		Section:              result.Section,
		ReplacementMarkdown:  result.ReplacementMarkdown,
		UpdatedDraftMarkdown: result.UpdatedDraftMarkdown,
		RegenerationID:       regenerationID,
	})
}

func draftContextFromRequest(styleProfileID, sessionID, personaID, formatID string) (authordomain.AuthorStyleProfile, authordomain.WritingStyleGuide, briefdomain.ArticleBrief, personadomain.Persona, outputformat.OutputFormat, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if strings.TrimSpace(styleProfileID) == "" && sessionID != "" {
		if session, ok := workflowStore.GetSession(sessionID); ok {
			styleProfileID = session.StyleProfileID
		}
	}
	profile, guide, ok := workflowStore.GetProfileAndGuide(styleProfileID)
	if !ok {
		return authordomain.AuthorStyleProfile{}, authordomain.WritingStyleGuide{}, briefdomain.ArticleBrief{}, personadomain.Persona{}, outputformat.OutputFormat{}, false
	}
	articleBrief, ok := workflowStore.GetBrief(sessionID)
	if !ok {
		session, sessionOK := workflowStore.GetSession(sessionID)
		if !sessionOK || !session.Completed {
			return authordomain.AuthorStyleProfile{}, authordomain.WritingStyleGuide{}, briefdomain.ArticleBrief{}, personadomain.Persona{}, outputformat.OutputFormat{}, false
		}
		articleBrief = session.AssembleBrief()
	}
	if strings.TrimSpace(personaID) == "" {
		personaID = articleBrief.PersonaID
	}
	persona, ok := resolvePersona(personaID)
	if !ok {
		return authordomain.AuthorStyleProfile{}, authordomain.WritingStyleGuide{}, briefdomain.ArticleBrief{}, personadomain.Persona{}, outputformat.OutputFormat{}, false
	}
	if strings.TrimSpace(formatID) == "" {
		formatID = articleBrief.OutputFormatID
	}
	if strings.TrimSpace(formatID) == "" {
		formatID = persona.DefaultFormat
	}
	format, ok := outputformat.DefaultRegistry().Get(formatID)
	if !ok {
		return authordomain.AuthorStyleProfile{}, authordomain.WritingStyleGuide{}, briefdomain.ArticleBrief{}, personadomain.Persona{}, outputformat.OutputFormat{}, false
	}
	articleBrief.PersonaID = persona.ID
	articleBrief.OutputFormatID = format.ID
	return profile, guide, articleBrief, persona, format, true
}

func historyDraftFromPath(pathID string) (sqliterepo.DraftRecord, bool) {
	if strings.TrimSpace(pathID) == "" {
		return sqliterepo.DraftRecord{}, false
	}
	store, ok := workflowStore.(workflowHistoryReader)
	if !ok {
		return sqliterepo.DraftRecord{}, false
	}
	return store.GetDraft(pathID)
}

func saveGeneratedDraftHistory(req generateDraftRequest, result draftapp.GenerateResult, articleBrief briefdomain.ArticleBrief, persona personadomain.Persona, format outputformat.OutputFormat) string {
	store, ok := workflowStore.(workflowHistoryWriter)
	if !ok {
		return ""
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		return ""
	}
	now := time.Now().UTC()
	projectID := historyRecordID("project", sessionID)
	articleID := historyRecordID("article", sessionID)
	title := briefTitle(sessionID, articleBrief)
	projectName := firstNonEmpty(title, sessionID)

	project := sqliterepo.ProjectRecord{
		ID:        projectID,
		Name:      projectName,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  map[string]any{"session_id": sessionID},
	}
	if existing, ok := store.GetProject(projectID); ok {
		project.CreatedAt = existing.CreatedAt
		if strings.TrimSpace(existing.Name) != "" {
			project.Name = existing.Name
		}
		project.Metadata = mergeHistoryMetadata(existing.Metadata, project.Metadata)
	}
	if err := store.SaveProject(project); err != nil {
		return ""
	}

	drafts, err := store.ListDrafts(articleID)
	if err != nil {
		return ""
	}
	draftID := newID("draft")
	version := len(drafts) + 1
	article := sqliterepo.ArticleRecord{
		ID:             articleID,
		ProjectID:      projectID,
		PersonaID:      persona.ID,
		OutputFormatID: format.ID,
		BriefSessionID: sessionID,
		CurrentDraftID: draftID,
		Title:          title,
		CreatedAt:      now,
		UpdatedAt:      now,
		Metadata:       map[string]any{"session_id": sessionID},
	}
	if existing, ok := store.GetArticle(articleID); ok {
		article.CreatedAt = existing.CreatedAt
		article.Metadata = mergeHistoryMetadata(existing.Metadata, article.Metadata)
	}
	if err := store.SaveArticle(article); err != nil {
		return ""
	}
	draft := sqliterepo.DraftRecord{
		ID:             draftID,
		ArticleID:      articleID,
		SessionID:      sessionID,
		StyleProfileID: firstNonEmpty(req.StyleProfileID, articleBrief.StyleProfileID),
		PersonaID:      persona.ID,
		OutputFormatID: format.ID,
		Version:        version,
		Markdown:       result.Draft.Markdown(),
		Evaluation:     result.Evaluation,
		Verification:   result.Verification,
		CreatedAt:      now,
	}
	if err := store.SaveDraft(draft); err != nil {
		return ""
	}
	return draftID
}

func saveSectionRegenerationHistory(baseDraft sqliterepo.DraftRecord, hasBaseDraft bool, req regenerateDraftSectionRequest, result draftapp.RegenerateSectionResult) string {
	if !hasBaseDraft {
		return ""
	}
	store, ok := workflowStore.(workflowHistoryWriter)
	if !ok {
		return ""
	}
	regenerations, err := store.ListSectionRegenerations(baseDraft.ID)
	if err != nil {
		return ""
	}
	now := time.Now().UTC()
	record := sqliterepo.SectionRegenerationRecord{
		ID:                   newID("regen"),
		DraftID:              baseDraft.ID,
		ArticleID:            baseDraft.ArticleID,
		SectionAnchor:        firstNonEmpty(req.SectionAnchor, result.Section.Anchor),
		SectionHeading:       result.Section.Heading,
		BaseVersion:          baseDraft.Version,
		Version:              len(regenerations) + baseDraft.Version + 1,
		ReplacementMarkdown:  result.ReplacementMarkdown,
		UpdatedDraftMarkdown: result.UpdatedDraftMarkdown,
		CreatedAt:            now,
	}
	if record.BaseVersion <= 0 {
		record.BaseVersion = 1
	}
	if record.Version <= record.BaseVersion {
		record.Version = record.BaseVersion + 1
	}
	if err := store.SaveSectionRegeneration(record); err != nil {
		return ""
	}
	return record.ID
}

func historyRecordID(prefix, seed string) string {
	seed = strings.TrimSpace(seed)
	if seed == "" {
		return newID(prefix)
	}
	return prefix + "_" + hex.EncodeToString([]byte(seed))
}

func mergeHistoryMetadata(existing, next map[string]any) map[string]any {
	if len(existing) == 0 {
		return next
	}
	merged := make(map[string]any, len(existing)+len(next))
	for key, value := range existing {
		merged[key] = value
	}
	for key, value := range next {
		merged[key] = value
	}
	return merged
}

func newDraftServiceWithVerifier(generator draftapp.TextGenerator, model string) (*draftapp.Service, error) {
	verifierClient, err := llamacpp.NewClientFromEnvForPurposeWithModel("VERIFY", model)
	if err != nil {
		return nil, err
	}
	return draftapp.NewServiceWithVerifier(generator, draftapp.NewLightweightVerifier(verifierClient)), nil
}

func decodeJSONRequest(r *http.Request, out any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(out)
}

func decodeBriefUpdateFields(r *http.Request) (map[string]string, error) {
	defer r.Body.Close()
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if fieldsRaw, ok := raw["fields"]; ok {
		var fields map[string]string
		if err := json.Unmarshal(fieldsRaw, &fields); err != nil {
			return nil, fmt.Errorf("fields must be an object of string values")
		}
		rawFields := make(map[string]json.RawMessage, len(fields))
		for key, value := range fields {
			encoded, _ := json.Marshal(value)
			rawFields[key] = encoded
		}
		raw = rawFields
	}
	fields := make(map[string]string, len(raw))
	for key, value := range raw {
		if key == "fields" {
			continue
		}
		var content string
		if err := json.Unmarshal(value, &content); err != nil {
			return nil, fmt.Errorf("field %q must be a string", key)
		}
		fields[key] = content
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("at least one brief field is required")
	}
	return fields, nil
}

func applyBriefUpdateFields(articleBrief *briefdomain.ArticleBrief, fields map[string]string) error {
	for key, value := range fields {
		switch normalizeBriefFieldName(key) {
		case "style_profile_id":
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("style_profile_id cannot be empty")
			}
			if _, _, ok := workflowStore.GetProfileAndGuide(value); !ok {
				return fmt.Errorf("style_profile_id was not found")
			}
			articleBrief.StyleProfileID = strings.TrimSpace(value)
		case "persona_id":
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("persona_id cannot be empty")
			}
			persona, ok := resolvePersona(value)
			if !ok {
				return fmt.Errorf("persona_id was not found")
			}
			articleBrief.PersonaID = persona.ID
		case "output_format_id":
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("output_format_id cannot be empty")
			}
			format, ok := outputformat.DefaultRegistry().Get(value)
			if !ok {
				return fmt.Errorf("output_format_id was not found")
			}
			articleBrief.OutputFormatID = format.ID
		case "theme":
			articleBrief.Theme = strings.TrimSpace(value)
		case "opening_episode":
			articleBrief.OpeningEpisode = strings.TrimSpace(value)
		case "reader":
			articleBrief.Reader = strings.TrimSpace(value)
		case "expected_reader_action":
			articleBrief.ExpectedReaderAction = strings.TrimSpace(value)
		case "must_include":
			articleBrief.MustInclude = strings.TrimSpace(value)
		case "personal_context":
			articleBrief.PersonalContext = strings.TrimSpace(value)
		case "exclusions":
			articleBrief.Exclusions = strings.TrimSpace(value)
		case "target_length_structure":
			articleBrief.TargetLengthStructure = strings.TrimSpace(value)
		case "tone_stance":
			articleBrief.ToneStance = strings.TrimSpace(value)
		default:
			return fmt.Errorf("unsupported brief field %q", key)
		}
	}
	for _, required := range []struct {
		name  string
		value string
	}{
		{"style_profile_id", articleBrief.StyleProfileID},
		{"persona_id", articleBrief.PersonaID},
		{"output_format_id", articleBrief.OutputFormatID},
		{"theme", articleBrief.Theme},
		{"reader", articleBrief.Reader},
		{"expected_reader_action", articleBrief.ExpectedReaderAction},
		{"must_include", articleBrief.MustInclude},
		{"personal_context", articleBrief.PersonalContext},
		{"target_length_structure", articleBrief.TargetLengthStructure},
	} {
		if strings.TrimSpace(required.value) == "" {
			return fmt.Errorf("%s cannot be empty", required.name)
		}
	}
	return nil
}

func normalizeBriefFieldName(name string) string {
	name = strings.TrimSpace(name)
	if alias, ok := map[string]string{
		"StyleProfileID":        "style_profile_id",
		"PersonaID":             "persona_id",
		"OutputFormatID":        "output_format_id",
		"Theme":                 "theme",
		"OpeningEpisode":        "opening_episode",
		"Reader":                "reader",
		"ExpectedReaderAction":  "expected_reader_action",
		"MustInclude":           "must_include",
		"PersonalContext":       "personal_context",
		"Exclusions":            "exclusions",
		"TargetLengthStructure": "target_length_structure",
		"ToneStance":            "tone_stance",
	}[name]; ok {
		return alias
	}
	var builder strings.Builder
	for i, r := range name {
		if r == '-' || r == ' ' {
			builder.WriteRune('_')
			continue
		}
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				builder.WriteRune('_')
			}
			builder.WriteRune(r + ('a' - 'A'))
			continue
		}
		builder.WriteRune(r)
	}
	return strings.ToLower(builder.String())
}

func styleGuideVersionFromRequest(base authorstyleapp.AnalyzeResult, req updateStyleGuideRequest) (authorstyleapp.AnalyzeResult, error) {
	guide := base.Guide
	changed := false
	if value := strings.TrimSpace(req.PreferredFirstPerson); value != "" {
		guide.PreferredFirstPerson = value
		changed = true
	}
	if len(req.RecurringThemes) > 0 {
		guide.RecurringThemes = cleanStringSlice(req.RecurringThemes)
		changed = true
	}
	if value := strings.TrimSpace(req.ParagraphRhythm); value != "" {
		guide.ParagraphRhythm = value
		changed = true
	}
	if value := strings.TrimSpace(req.SentenceRhythm); value != "" {
		guide.SentenceRhythm = value
		changed = true
	}
	if value := strings.TrimSpace(req.HeadingGuidance); value != "" {
		guide.HeadingGuidance = value
		changed = true
	}
	if value := strings.TrimSpace(req.QuoteGuidance); value != "" {
		guide.QuoteGuidance = value
		changed = true
	}
	if len(req.OpeningPatterns) > 0 {
		guide.OpeningPatterns = cleanStringSlice(req.OpeningPatterns)
		changed = true
	}
	if len(req.ConclusionPatterns) > 0 {
		guide.ConclusionPatterns = cleanStringSlice(req.ConclusionPatterns)
		changed = true
	}
	if len(req.Warnings) > 0 {
		guide.Warnings = cleanStringSlice(req.Warnings)
		changed = true
	}
	if markdown := strings.TrimSpace(req.GuideMarkdown); markdown != "" {
		guide.Markdown = markdown
		changed = true
	} else if changed {
		guide.Markdown = authordomain.GuideMarkdown(guide)
	}
	if !changed {
		return authorstyleapp.AnalyzeResult{}, fmt.Errorf("at least one style guide field is required")
	}
	if err := guide.Validate(); err != nil {
		return authorstyleapp.AnalyzeResult{}, err
	}
	suffix := strings.TrimPrefix(newID("edit"), "edit_")
	guide.ID = firstNonEmpty(base.Guide.ID, "guide") + "_edit_" + suffix
	updated := base
	updated.ID = firstNonEmpty(base.ID, "author_style") + "_edit_" + suffix
	updated.Guide = guide
	updated.CreatedAt = time.Now().UTC()
	return updated, nil
}

func cleanStringSlice(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			cleaned = append(cleaned, value)
		}
	}
	return cleaned
}

func wantsEventStream(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}

type sseStream struct {
	w       http.ResponseWriter
	flusher http.Flusher
	started time.Time
	mu      sync.Mutex
}

type streamStatus struct {
	Status    string  `json:"status"`
	Phase     string  `json:"phase,omitempty"`
	Endpoint  string  `json:"endpoint,omitempty"`
	Model     string  `json:"model,omitempty"`
	StartedAt string  `json:"started_at,omitempty"`
	ElapsedMS int64   `json:"elapsed_ms"`
	Runes     int     `json:"runes,omitempty"`
	Score     float64 `json:"score,omitempty"`
}

type streamChunk struct {
	Text      string `json:"text"`
	ElapsedMS int64  `json:"elapsed_ms"`
}

type streamError struct {
	Code         string                      `json:"code"`
	Message      string                      `json:"message"`
	Detail       string                      `json:"detail,omitempty"`
	ElapsedMS    int64                       `json:"elapsed_ms"`
	Draft        string                      `json:"draft,omitempty"`
	DraftPath    string                      `json:"draft_path,omitempty"`
	Evaluation   *draftapp.StyleEvaluation   `json:"evaluation,omitempty"`
	Verification *draftapp.FinalVerification `json:"verification,omitempty"`
	QualityGate  *draftQualityGateDetails    `json:"quality_gate,omitempty"`
}

func newSSEStream(w http.ResponseWriter) (*sseStream, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream; charset=utf-8")
	headers.Set("Cache-Control", "no-cache, no-transform")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	stream := &sseStream{w: w, flusher: flusher, started: time.Now()}
	_ = stream.Send("status", streamStatus{Status: "stream_opened", ElapsedMS: 0})
	return stream, true
}

func (s *sseStream) ElapsedMS() int64 {
	return time.Since(s.started).Milliseconds()
}

func (s *sseStream) Send(event string, data any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, encoded); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *sseStream) StartHeartbeat(ctx context.Context, phase, endpoint, model string, interval time.Duration) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				_ = s.Send("heartbeat", streamStatus{
					Status:    "running",
					Phase:     phase,
					Endpoint:  endpoint,
					Model:     model,
					StartedAt: s.started.Format(time.RFC3339),
					ElapsedMS: s.ElapsedMS(),
				})
			}
		}
	}()
	return func() {
		close(done)
	}
}

func pathValue(r *http.Request, name string) string {
	if value := strings.TrimSpace(r.PathValue(name)); value != "" {
		return value
	}
	return strings.TrimSpace(mux.Vars(r)[name])
}

func toAuthorStyleResponse(result authorstyleapp.AnalyzeResult) authorStyleResponse {
	return authorStyleResponse{
		ID:            result.ID,
		ProfileID:     result.Profile.ID,
		GuideID:       result.Guide.ID,
		GuideMarkdown: result.Guide.Markdown,
		ArticleCount:  result.ArticleCount,
		CreatedAt:     formatOptionalTime(result.CreatedAt),
		Source:        result.Source,
		Profile:       result.Profile,
		Guide:         result.Guide,
	}
}

func toStyleGuideArtifactResponses(results []authorstyleapp.AnalyzeResult) []styleGuideArtifactResponse {
	items := make([]styleGuideArtifactResponse, 0, len(results))
	for _, result := range results {
		items = append(items, toStyleGuideArtifactResponse(result))
	}
	return items
}

func toStyleGuideArtifactResponse(result authorstyleapp.AnalyzeResult) styleGuideArtifactResponse {
	return styleGuideArtifactResponse{
		ID:            result.Guide.ID,
		AnalysisID:    result.ID,
		ProfileID:     result.Profile.ID,
		GuideID:       result.Guide.ID,
		Title:         styleGuideTitle(result),
		Description:   styleGuideDescription(result),
		CreatedAt:     formatOptionalTime(result.CreatedAt),
		ArticleCount:  result.ArticleCount,
		GuideMarkdown: result.Guide.Markdown,
		Source:        result.Source,
		Profile:       result.Profile,
		Guide:         result.Guide,
	}
}

func sortAuthorStyles(results []authorstyleapp.AnalyzeResult) {
	sort.SliceStable(results, func(i, j int) bool {
		left := results[i].CreatedAt
		right := results[j].CreatedAt
		if !left.Equal(right) {
			return left.After(right)
		}
		return results[i].ID < results[j].ID
	})
}

func styleGuideTitle(result authorstyleapp.AnalyzeResult) string {
	if username := strings.TrimSpace(result.Source.Username); username != "" {
		return username
	}
	if len(result.Source.Articles) > 0 {
		if title := strings.TrimSpace(result.Source.Articles[0].Title); title != "" {
			return title
		}
	}
	return firstNonEmpty(result.Profile.ID, result.Guide.ID, result.ID)
}

func styleGuideDescription(result authorstyleapp.AnalyzeResult) string {
	if len(result.Source.Articles) == 0 {
		return ""
	}
	titles := make([]string, 0, len(result.Source.Articles))
	for _, article := range result.Source.Articles {
		if title := strings.TrimSpace(article.Title); title != "" {
			titles = append(titles, title)
		}
		if len(titles) == 3 {
			break
		}
	}
	return strings.Join(titles, " / ")
}

func toBriefSessionResponse(result briefapp.InterviewResult) briefSessionResponse {
	var question *articleQuestionJSON
	if result.NextQuestion != nil {
		value := toArticleQuestionJSON(*result.NextQuestion)
		question = &value
	}
	return briefSessionResponse{
		SessionID:       result.Session.ID,
		StyleProfileID:  result.Session.StyleProfileID,
		PersonaID:       result.Session.PersonaID,
		OutputFormatID:  result.Session.OutputFormatID,
		ParentSessionID: result.Session.ParentSessionID,
		Phase:           string(result.Session.Phase),
		Completed:       result.Completed || result.Session.Completed,
		NextQuestion:    question,
		Brief:           result.Brief,
		Answers:         result.Session.Answers,
	}
}

func toBriefSessionSummaryResponses(sessions []briefdomain.ArticleBriefSession, briefs map[string]briefdomain.ArticleBrief) []briefSessionSummaryResponse {
	items := make([]briefSessionSummaryResponse, 0, len(sessions))
	for _, session := range sessions {
		articleBrief, briefAvailable := briefs[session.ID]
		items = append(items, toBriefSessionSummaryResponse(session, articleBrief, briefAvailable))
	}
	return items
}

func toBriefSessionSummaryResponse(session briefdomain.ArticleBriefSession, articleBrief briefdomain.ArticleBrief, briefAvailable bool) briefSessionSummaryResponse {
	return briefSessionSummaryResponse{
		SessionID:       session.ID,
		StyleProfileID:  session.StyleProfileID,
		PersonaID:       session.PersonaID,
		OutputFormatID:  session.OutputFormatID,
		ParentSessionID: session.ParentSessionID,
		Phase:           string(session.Phase),
		Completed:       session.Completed,
		AnswerCount:     len(session.Answers),
		QuestionCount:   len(session.Questions),
		BriefAvailable:  briefAvailable,
		Title:           briefTitle(session.ID, articleBrief),
	}
}

func toBriefVersionResponses(versions []briefdomain.ArticleBriefVersion) []briefVersionResponse {
	items := make([]briefVersionResponse, 0, len(versions))
	currentVersion := currentBriefVersion(versions)
	for _, version := range versions {
		items = append(items, briefVersionResponse{
			SessionID:      version.SessionID,
			Version:        version.Version,
			Current:        version.Version == currentVersion,
			CreatedAt:      formatOptionalTime(version.CreatedAt),
			Title:          briefTitle(version.SessionID, version.Brief),
			StyleProfileID: version.Brief.StyleProfileID,
			PersonaID:      version.Brief.PersonaID,
			OutputFormatID: version.Brief.OutputFormatID,
			Brief:          version.Brief,
		})
	}
	return items
}

func currentBriefVersion(versions []briefdomain.ArticleBriefVersion) int {
	current := 0
	for _, version := range versions {
		if version.Version > current {
			current = version.Version
		}
	}
	return current
}

func sortBriefSessions(sessions []briefdomain.ArticleBriefSession) {
	sort.SliceStable(sessions, func(i, j int) bool {
		if sessions[i].Completed != sessions[j].Completed {
			return sessions[i].Completed
		}
		return sessions[i].ID < sessions[j].ID
	})
}

func toProjectHistoryResponses(projects []sqliterepo.ProjectRecord) []projectHistoryResponse {
	items := make([]projectHistoryResponse, 0, len(projects))
	for _, project := range projects {
		items = append(items, toProjectHistoryResponse(project))
	}
	return items
}

func toProjectHistoryResponse(project sqliterepo.ProjectRecord) projectHistoryResponse {
	return projectHistoryResponse{
		ID:        project.ID,
		Name:      project.Name,
		CreatedAt: formatOptionalTime(project.CreatedAt),
		UpdatedAt: formatOptionalTime(project.UpdatedAt),
		Metadata:  project.Metadata,
	}
}

func listWorkflowHistoryResponses(store workflowHistoryReader) ([]projectHistoryResponse, []articleHistoryResponse, []draftHistoryResponse, error) {
	projects, err := store.ListProjects()
	if err != nil {
		return nil, nil, nil, err
	}
	projectResponses := make([]projectHistoryResponse, 0, len(projects))
	articleResponses := []articleHistoryResponse{}
	draftResponses := []draftHistoryResponse{}
	for _, project := range projects {
		projectResponse := toProjectHistoryResponse(project)
		snapshots, err := store.ListSourceSnapshots("project", project.ID)
		if err != nil {
			return nil, nil, nil, err
		}
		projectResponse.SourceSnapshots = toSourceSnapshotHistoryResponses(snapshots)
		articles, err := store.ListArticlesByProject(project.ID)
		if err != nil {
			return nil, nil, nil, err
		}
		projectResponse.Articles = make([]articleHistoryResponse, 0, len(articles))
		for _, article := range articles {
			articleResponse, drafts, err := articleHistoryResponseWithDetails(store, article)
			if err != nil {
				return nil, nil, nil, err
			}
			projectResponse.Articles = append(projectResponse.Articles, articleResponse)
			articleResponses = append(articleResponses, articleResponse)
			draftResponses = append(draftResponses, drafts...)
		}
		projectResponses = append(projectResponses, projectResponse)
	}
	return projectResponses, articleResponses, draftResponses, nil
}

func articleHistoryResponseWithDetails(store workflowHistoryReader, article sqliterepo.ArticleRecord) (articleHistoryResponse, []draftHistoryResponse, error) {
	articleResponse := toArticleHistoryResponse(article)
	snapshots, err := store.ListSourceSnapshots("article", article.ID)
	if err != nil {
		return articleHistoryResponse{}, nil, err
	}
	articleResponse.SourceSnapshots = toSourceSnapshotHistoryResponses(snapshots)
	drafts, err := store.ListDrafts(article.ID)
	if err != nil {
		return articleHistoryResponse{}, nil, err
	}
	draftResponses := make([]draftHistoryResponse, 0, len(drafts))
	for _, draft := range drafts {
		draftResponse, err := draftHistoryResponseWithDetails(store, draft)
		if err != nil {
			return articleHistoryResponse{}, nil, err
		}
		draftResponses = append(draftResponses, draftResponse)
	}
	articleResponse.Drafts = draftResponses
	return articleResponse, draftResponses, nil
}

func draftHistoryResponseWithDetails(store workflowHistoryReader, draft sqliterepo.DraftRecord) (draftHistoryResponse, error) {
	draftResponse := toDraftHistoryResponse(draft)
	snapshots, err := store.ListSourceSnapshots("draft", draft.ID)
	if err != nil {
		return draftHistoryResponse{}, err
	}
	draftResponse.SourceSnapshots = toSourceSnapshotHistoryResponses(snapshots)
	regenerations, err := store.ListSectionRegenerations(draft.ID)
	if err != nil {
		return draftHistoryResponse{}, err
	}
	draftResponse.SectionRegenerations = toSectionRegenerationHistoryResponses(regenerations)
	return draftResponse, nil
}

func toArticleHistoryResponse(article sqliterepo.ArticleRecord) articleHistoryResponse {
	return articleHistoryResponse{
		ID:             article.ID,
		ProjectID:      article.ProjectID,
		PersonaID:      article.PersonaID,
		OutputFormatID: article.OutputFormatID,
		BriefSessionID: article.BriefSessionID,
		CurrentDraftID: article.CurrentDraftID,
		Title:          article.Title,
		CreatedAt:      formatOptionalTime(article.CreatedAt),
		UpdatedAt:      formatOptionalTime(article.UpdatedAt),
		Metadata:       article.Metadata,
	}
}

func toSourceSnapshotHistoryResponses(snapshots []sqliterepo.SourceSnapshotRecord) []sourceSnapshotHistoryResponse {
	items := make([]sourceSnapshotHistoryResponse, 0, len(snapshots))
	for _, snapshot := range snapshots {
		items = append(items, toSourceSnapshotHistoryResponse(snapshot))
	}
	return items
}

func toSourceSnapshotHistoryResponse(snapshot sqliterepo.SourceSnapshotRecord) sourceSnapshotHistoryResponse {
	return sourceSnapshotHistoryResponse{
		ID:          snapshot.ID,
		ScopeType:   snapshot.ScopeType,
		ScopeID:     snapshot.ScopeID,
		Selector:    snapshot.Selector,
		Profile:     snapshot.Profile,
		Article:     snapshot.Article,
		ContentHash: snapshot.ContentHash,
		FetchedAt:   formatOptionalTime(snapshot.FetchedAt),
		CreatedAt:   formatOptionalTime(snapshot.CreatedAt),
	}
}

func toDraftHistoryResponses(drafts []sqliterepo.DraftRecord) []draftHistoryResponse {
	items := make([]draftHistoryResponse, 0, len(drafts))
	for _, draft := range drafts {
		items = append(items, toDraftHistoryResponse(draft))
	}
	return items
}

func toDraftHistoryResponse(draft sqliterepo.DraftRecord) draftHistoryResponse {
	return draftHistoryResponse{
		ID:                      draft.ID,
		ArticleID:               draft.ArticleID,
		SessionID:               draft.SessionID,
		StyleProfileID:          draft.StyleProfileID,
		PersonaID:               draft.PersonaID,
		OutputFormatID:          draft.OutputFormatID,
		Version:                 draft.Version,
		Markdown:                draft.Markdown,
		ContentHash:             draft.ContentHash,
		Evaluation:              draft.Evaluation,
		Verification:            draft.Verification,
		QuestionTemplateVersion: draft.QuestionTemplateVersion,
		CreatedAt:               formatOptionalTime(draft.CreatedAt),
	}
}

func toSectionRegenerationHistoryResponses(regenerations []sqliterepo.SectionRegenerationRecord) []sectionRegenerationHistoryResponse {
	items := make([]sectionRegenerationHistoryResponse, 0, len(regenerations))
	for _, regeneration := range regenerations {
		items = append(items, toSectionRegenerationHistoryResponse(regeneration))
	}
	return items
}

func toSectionRegenerationHistoryResponse(regeneration sqliterepo.SectionRegenerationRecord) sectionRegenerationHistoryResponse {
	return sectionRegenerationHistoryResponse{
		ID:                   regeneration.ID,
		DraftID:              regeneration.DraftID,
		ArticleID:            regeneration.ArticleID,
		SectionAnchor:        regeneration.SectionAnchor,
		SectionHeading:       regeneration.SectionHeading,
		BaseVersion:          regeneration.BaseVersion,
		Version:              regeneration.Version,
		ReplacementMarkdown:  regeneration.ReplacementMarkdown,
		UpdatedDraftMarkdown: regeneration.UpdatedDraftMarkdown,
		UpdatedContentHash:   regeneration.UpdatedContentHash,
		Verification:         regeneration.Verification,
		CreatedAt:            formatOptionalTime(regeneration.CreatedAt),
	}
}

func listBriefArtifactResponses(briefs map[string]briefdomain.ArticleBrief) []briefArtifactResponse {
	sessionIDs := make([]string, 0, len(briefs))
	for sessionID := range briefs {
		sessionIDs = append(sessionIDs, sessionID)
	}
	sort.Strings(sessionIDs)
	items := make([]briefArtifactResponse, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		session, sessionOK := workflowStore.GetSession(sessionID)
		items = append(items, toBriefArtifactResponse(sessionID, briefs[sessionID], session, sessionOK))
	}
	return items
}

func toBriefArtifactResponse(sessionID string, articleBrief briefdomain.ArticleBrief, session briefdomain.ArticleBriefSession, hasSession bool) briefArtifactResponse {
	answerCount := 0
	parentSessionID := ""
	if hasSession {
		answerCount = len(session.Answers)
		parentSessionID = session.ParentSessionID
	}
	return briefArtifactResponse{
		SessionID:       sessionID,
		StyleProfileID:  articleBrief.StyleProfileID,
		PersonaID:       articleBrief.PersonaID,
		OutputFormatID:  articleBrief.OutputFormatID,
		ParentSessionID: parentSessionID,
		Title:           briefTitle(sessionID, articleBrief),
		Description:     briefDescription(articleBrief),
		AnswerCount:     answerCount,
		DeepDiveCount:   len(articleBrief.DeepDives),
		Brief:           articleBrief,
	}
}

func briefTitle(sessionID string, articleBrief briefdomain.ArticleBrief) string {
	return firstNonEmpty(articleBrief.Theme, articleBrief.OpeningEpisode, articleBrief.Reader, sessionID)
}

func briefDescription(articleBrief briefdomain.ArticleBrief) string {
	return firstNonEmpty(articleBrief.ExpectedReaderAction, articleBrief.MustInclude, articleBrief.PersonalContext)
}

func toArticleQuestionJSONList(questions []briefdomain.ArticleQuestion) []articleQuestionJSON {
	result := make([]articleQuestionJSON, 0, len(questions))
	for _, question := range questions {
		result = append(result, toArticleQuestionJSON(question))
	}
	return result
}

func toArticleQuestionJSON(question briefdomain.ArticleQuestion) articleQuestionJSON {
	return articleQuestionJSON{
		ID:               question.ID,
		Text:             question.Text,
		FlowType:         string(question.FlowType),
		TargetField:      question.TargetField,
		TargetQuestionID: question.TargetQuestionID,
		FollowUpIndex:    question.FollowUpIndex,
	}
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func newID(prefix string) string {
	var bytes [6]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%s_fallback", prefix)
	}
	return prefix + "_" + hex.EncodeToString(bytes[:])
}

func newBriefInterviewService(model string) *briefapp.InterviewService {
	return newBriefInterviewServiceWithStyleGuide(model, "")
}

func newBriefInterviewServiceForSession(session briefdomain.ArticleBriefSession, model string) *briefapp.InterviewService {
	_, guide, ok := workflowStore.GetProfileAndGuide(session.StyleProfileID)
	if !ok {
		return newBriefInterviewService(model)
	}
	return newBriefInterviewServiceWithStyleGuide(model, guide.Markdown)
}

func newBriefInterviewServiceWithStyleGuide(model, styleGuideMarkdown string) *briefapp.InterviewService {
	return briefapp.NewInterviewService(llmFollowUpGenerator{
		model:              strings.TrimSpace(model),
		styleGuideMarkdown: strings.TrimSpace(styleGuideMarkdown),
	})
}

type llmFollowUpGenerator struct {
	model              string
	styleGuideMarkdown string
}

func (g llmFollowUpGenerator) GenerateFollowUp(ctx context.Context, session briefdomain.ArticleBriefSession, target briefdomain.ArticleQuestion, answer briefdomain.BriefAnswer, followUpIndex int) (string, error) {
	generator, err := llamacpp.NewClientFromEnvForPurposeWithModel("BRIEF", g.model)
	if err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	prompt := buildFollowUpPrompt(session, target, answer, followUpIndex, g.styleGuideMarkdown)
	generated, err := generator.Generate(ctx, prompt)
	if err != nil {
		return "", err
	}
	question := extractFollowUpQuestion(generated)
	if !briefdomain.IsAllowedFollowUpQuestion(question) {
		return "", fmt.Errorf("generated follow-up was not allowed")
	}
	return question, nil
}

func (g llmFollowUpGenerator) GenerateFollowUpStream(ctx context.Context, session briefdomain.ArticleBriefSession, target briefdomain.ArticleQuestion, answer briefdomain.BriefAnswer, followUpIndex int, onChunk func(string) error) (string, error) {
	generator, err := llamacpp.NewClientFromEnvForPurposeWithModel("BRIEF", g.model)
	if err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	prompt := buildFollowUpPrompt(session, target, answer, followUpIndex, g.styleGuideMarkdown)
	generated, err := generator.GenerateStream(ctx, prompt, onChunk)
	if err != nil {
		return "", err
	}
	question := extractFollowUpQuestion(generated)
	if !briefdomain.IsAllowedFollowUpQuestion(question) {
		return "", fmt.Errorf("generated follow-up was not allowed")
	}
	return question, nil
}

func toDomainQuestions(values []articleQuestionJSON) []briefdomain.ArticleQuestion {
	if len(values) == 0 {
		return nil
	}
	questions := make([]briefdomain.ArticleQuestion, 0, len(values))
	for _, value := range values {
		flowType := briefdomain.QuestionFlowType(value.FlowType)
		if flowType == "" {
			flowType = briefdomain.QuestionFlowMain
		}
		questions = append(questions, briefdomain.ArticleQuestion{
			ID:          strings.TrimSpace(value.ID),
			Text:        strings.TrimSpace(value.Text),
			FlowType:    flowType,
			Required:    requiredForQuestionID(value.ID),
			TargetField: targetFieldForQuestion(value),
		})
	}
	return briefdomain.NormalizeQuestions(questions)
}

func requiredForQuestionID(id string) bool {
	for _, question := range briefdomain.FixedQuestions() {
		if question.ID == id {
			return question.Required
		}
	}
	return false
}

func targetFieldForQuestion(value articleQuestionJSON) string {
	if field := strings.TrimSpace(value.TargetField); field != "" {
		return field
	}
	for _, question := range briefdomain.FixedQuestions() {
		if question.ID == value.ID {
			return question.TargetField
		}
	}
	return "custom"
}

func buildFollowUpPrompt(session briefdomain.ArticleBriefSession, target briefdomain.ArticleQuestion, answer briefdomain.BriefAnswer, followUpIndex int, styleGuideMarkdown string) string {
	excerpt := followUpPromptExcerpt(answer.Content, 90)
	styleGuideMarkdown = strings.TrimSpace(styleGuideMarkdown)
	if styleGuideMarkdown == "" {
		styleGuideMarkdown = "文体ガイド未設定。セッションの既存回答に合わせ、自然な日本語で質問する。"
	}
	return fmt.Sprintf(`あなたは記事の取材編集者です。
次の記事ブリーフ回答を深掘りするため、著者がすぐ答えられる追加質問を1つだけ作ってください。

条件:
- 日本語で質問する
- はい/いいえで答えられる質問にしない
- 選択式にしない
- 抽象論ではなく、1つの具体的な場面、手順、数字、失敗、気持ち、判断理由のどれかを引き出す
- 質問は30〜70字程度にする
- 難しい編集用語を使わない
- 文体ガイドのトーンから外れない
- 質問文だけを出力する
- 質問は必ず「%s」というご回答を踏まえて、から始める

セッションID: %s
親質問: %s
親回答: %s
深掘り回数: %d
文体ガイド:
%s
`, excerpt, session.ID, target.Text, answer.Content, followUpIndex, styleGuideMarkdown)
}

func extractFollowUpQuestion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`\"'")
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "-*0123456789. "))
		line = strings.Trim(line, "\"'")
		if strings.HasSuffix(line, "？") || strings.HasSuffix(line, "?") {
			return line
		}
	}
	return value
}

func followUpPromptExcerpt(content string, maxRunes int) string {
	content = strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if content == "" {
		return "その点"
	}
	content = strings.Trim(content, "「」\"'")
	runes := []rune(content)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes-1]) + "..."
	}
	return content
}
