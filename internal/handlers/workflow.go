package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	authorstyleapp "github.com/teradakousuke/note_maker/internal/application/authorstyle"
	briefapp "github.com/teradakousuke/note_maker/internal/application/brief"
	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	"github.com/teradakousuke/note_maker/internal/infrastructure/llamacpp"
	notenote "github.com/teradakousuke/note_maker/internal/infrastructure/note"
	"github.com/teradakousuke/note_maker/internal/infrastructure/repository/memory"
)

var workflowStore = memory.NewWorkflowStore()

type analyzeAuthorStyleRequest struct {
	Username    string   `json:"username"`
	ArticleURLs []string `json:"article_urls"`
	Limit       int      `json:"limit"`
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
	StyleProfileID string `json:"style_profile_id"`
	SessionID      string `json:"session_id"`
}

type answerBriefSessionRequest struct {
	Content      string `json:"content"`
	SkipDeepDive bool   `json:"skip_deep_dive"`
}

type briefSessionResponse struct {
	SessionID      string                    `json:"session_id"`
	StyleProfileID string                    `json:"style_profile_id"`
	Phase          string                    `json:"phase"`
	Completed      bool                      `json:"completed"`
	NextQuestion   *articleQuestionJSON      `json:"next_question,omitempty"`
	Brief          *briefdomain.ArticleBrief `json:"brief,omitempty"`
	Answers        []briefdomain.BriefAnswer `json:"answers"`
}

type articleQuestionJSON struct {
	ID               string `json:"id"`
	Text             string `json:"text"`
	FlowType         string `json:"flow_type"`
	TargetQuestionID string `json:"target_question_id,omitempty"`
	FollowUpIndex    int    `json:"follow_up_index,omitempty"`
}

type generateDraftRequest struct {
	StyleProfileID string `json:"style_profile_id"`
	SessionID      string `json:"session_id"`
}

type generateDraftResponse struct {
	Draft      string                   `json:"draft"`
	Evaluation draftapp.StyleEvaluation `json:"evaluation"`
}

// AnalyzeAuthorStyleHandler analyzes a note author and stores the resulting style assets.
func AnalyzeAuthorStyleHandler(w http.ResponseWriter, r *http.Request) {
	var req analyzeAuthorStyleRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}

	service := authorstyleapp.NewAnalyzeAuthorStyleService(notenote.NewFetcher(), nil)
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

	respondWithJSON(w, http.StatusOK, toAuthorStyleResponse(result))
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

	service := briefapp.NewInterviewService(localFollowUpGenerator{})
	result, err := service.StartSession(briefapp.StartSessionInput{
		SessionID:      req.SessionID,
		StyleProfileID: req.StyleProfileID,
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
		service := briefapp.NewInterviewService(localFollowUpGenerator{})
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

	generator, err := llamacpp.NewClientFromEnv()
	if err != nil {
		respondWithError(w, "GENERATOR_INITIALIZATION_FAILED", "Failed to initialize local LLM client", err.Error(), http.StatusInternalServerError)
		return
	}
	service := draftapp.NewService(generator)
	result, err := service.Generate(r.Context(), draftapp.GenerateRequest{
		StyleGuide:    guide,
		Brief:         articleBrief,
		AuthorProfile: profile,
	})
	if err != nil {
		respondWithError(w, "DRAFT_GENERATION_FAILED", "Failed to generate draft", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusOK, generateDraftResponse{
		Draft:      result.Draft.Markdown(),
		Evaluation: result.Evaluation,
	})
}

func decodeJSONRequest(r *http.Request, out any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(out)
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
		question = &articleQuestionJSON{
			ID:               result.NextQuestion.ID,
			Text:             result.NextQuestion.Text,
			FlowType:         string(result.NextQuestion.FlowType),
			TargetQuestionID: result.NextQuestion.TargetQuestionID,
			FollowUpIndex:    result.NextQuestion.FollowUpIndex,
		}
	}
	return briefSessionResponse{
		SessionID:      result.Session.ID,
		StyleProfileID: result.Session.StyleProfileID,
		Phase:          string(result.Session.Phase),
		Completed:      result.Completed || result.Session.Completed,
		NextQuestion:   question,
		Brief:          result.Brief,
		Answers:        result.Session.Answers,
	}
}

func newID(prefix string) string {
	var bytes [6]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("%s_fallback", prefix)
	}
	return prefix + "_" + hex.EncodeToString(bytes[:])
}

type localFollowUpGenerator struct{}

func (localFollowUpGenerator) GenerateFollowUp(ctx context.Context, session briefdomain.ArticleBriefSession, target briefdomain.ArticleQuestion, answer briefdomain.BriefAnswer, followUpIndex int) (string, error) {
	return briefdomain.FallbackFollowUpText(target, answer, followUpIndex), nil
}
