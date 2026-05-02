package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
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
	SaveSession(briefdomain.ArticleBriefSession) error
	GetSession(string) (briefdomain.ArticleBriefSession, bool)
	SaveBrief(string, briefdomain.ArticleBrief) error
	GetBrief(string) (briefdomain.ArticleBrief, bool)
	GetProfileAndGuide(string) (authordomain.AuthorStyleProfile, authordomain.WritingStyleGuide, bool)
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
	Username    string   `json:"username"`
	ArticleURLs []string `json:"article_urls"`
	Limit       int      `json:"limit"`
	StyleModel  string   `json:"style_model"`
}

type seedAuthorStyleRequest struct {
	PersonaID string `json:"persona_id"`
}

type authorStyleResponse struct {
	ID            string `json:"id"`
	ProfileID     string `json:"profile_id"`
	GuideID       string `json:"guide_id"`
	GuideMarkdown string `json:"guide_markdown"`
	ArticleCount  int    `json:"article_count"`
	Source        any    `json:"source"`
	Profile       any    `json:"profile"`
	Guide         any    `json:"guide"`
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
	Evaluation   draftapp.StyleEvaluation   `json:"evaluation"`
	Verification draftapp.FinalVerification `json:"verification"`
}

type regenerateDraftSectionResponse struct {
	Section              draftapp.MarkdownSection `json:"section"`
	ReplacementMarkdown  string                   `json:"replacement_markdown"`
	UpdatedDraftMarkdown string                   `json:"updated_draft_markdown"`
}

// ListPersonasHandler returns built-in writing personas.
func ListPersonasHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, personadomain.DefaultRegistry().List())
}

// ListFormatsHandler returns built-in output formats.
func ListFormatsHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, outputformat.DefaultRegistry().List())
}

// GetBriefSessionTemplateHandler returns the composed fixed-question template.
func GetBriefSessionTemplateHandler(w http.ResponseWriter, r *http.Request) {
	persona, ok := personadomain.DefaultRegistry().Get(r.URL.Query().Get("persona_id"))
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
	persona, ok := personadomain.DefaultRegistry().Get(req.PersonaID)
	if !ok {
		respondWithError(w, "UNKNOWN_PERSONA", "Persona was not found", req.PersonaID, http.StatusBadRequest)
		return
	}
	result, err := buildPresetAuthorStyle(persona)
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

// AnalyzeAuthorStyleHandler analyzes a note author and stores the resulting style assets.
func AnalyzeAuthorStyleHandler(w http.ResponseWriter, r *http.Request) {
	var req analyzeAuthorStyleRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}

	service := authorstyleapp.NewAnalyzeAuthorStyleService(sourcefetch.NewAuthorStyleFetcher(), nil)
	result, err := service.Analyze(r.Context(), authorstyleapp.AnalyzeRequest{
		Username:    req.Username,
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

func buildPresetAuthorStyle(persona personadomain.Persona) (authorstyleapp.AnalyzeResult, error) {
	fetchedAt := time.Now().UTC()
	content := strings.Join([]string{
		persona.Description,
		persona.VoiceNotes.Tone,
		strings.Join(persona.VoiceNotes.FirstPerson, " "),
		strings.Join(persona.VoiceNotes.TitlePatterns, " "),
		strings.Repeat(" "+strings.Join(persona.VoiceNotes.AntiPatterns, " "), 2),
	}, "\n\n")
	if strings.TrimSpace(content) == "" {
		content = persona.DisplayName + " writing preset"
	}
	articleURL := "preset://" + persona.ID
	if len(persona.Sources) > 0 && strings.TrimSpace(persona.Sources[0].URL) != "" {
		articleURL = persona.Sources[0].URL
	}
	article := articledomain.Article{
		URL:     articleURL,
		Title:   persona.DisplayName + " preset",
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
	guide.ParagraphRhythm = persona.VoiceNotes.Tone
	guide.HeadingGuidance = "出力先の形式に合わせ、読者が流れを追いやすい見出しを置く"
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
	persona, ok := personadomain.DefaultRegistry().Get(req.PersonaID)
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
	persona, ok := personadomain.DefaultRegistry().Get(personaID)
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
		respondWithError(w, "DRAFT_GENERATION_FAILED", "Failed to generate draft", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusOK, generateDraftResponse{
		Draft:        result.Draft.Markdown(),
		Evaluation:   result.Evaluation,
		Verification: result.Verification,
	})
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
		_ = stream.Send("error", streamError{Code: "DRAFT_GENERATION_FAILED", Message: "Failed to generate draft", Detail: err.Error(), ElapsedMS: stream.ElapsedMS()})
		return
	}
	_ = stream.Send("result", generateDraftResponse{
		Draft:        result.Draft.Markdown(),
		Evaluation:   result.Evaluation,
		Verification: result.Verification,
	})
	_ = stream.Send("done", streamStatus{Status: "completed", Phase: "draft", Endpoint: endpoint, Model: model, StartedAt: stream.started.Format(time.RFC3339), ElapsedMS: stream.ElapsedMS(), Runes: len([]rune(result.Draft.Markdown())), Score: result.Evaluation.Comparison.Score})
}

// RegenerateDraftSectionHandler rewrites only one h2 subtree of the current draft.
func RegenerateDraftSectionHandler(w http.ResponseWriter, r *http.Request) {
	var req regenerateDraftSectionRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.SessionID) == "" {
		req.SessionID = pathValue(r, "id")
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
	respondWithJSON(w, http.StatusOK, regenerateDraftSectionResponse{
		Section:              result.Section,
		ReplacementMarkdown:  result.ReplacementMarkdown,
		UpdatedDraftMarkdown: result.UpdatedDraftMarkdown,
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
	persona, ok := personadomain.DefaultRegistry().Get(personaID)
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
	Code      string `json:"code"`
	Message   string `json:"message"`
	Detail    string `json:"detail,omitempty"`
	ElapsedMS int64  `json:"elapsed_ms"`
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
		Source:        result.Source,
		Profile:       result.Profile,
		Guide:         result.Guide,
	}
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
