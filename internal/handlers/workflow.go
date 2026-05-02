package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	authorstyleapp "github.com/teradakousuke/note_maker/internal/application/authorstyle"
	briefapp "github.com/teradakousuke/note_maker/internal/application/brief"
	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	"github.com/teradakousuke/note_maker/internal/infrastructure/llamacpp"
	notenote "github.com/teradakousuke/note_maker/internal/infrastructure/note"
	"github.com/teradakousuke/note_maker/internal/infrastructure/repository/memory"
)

var workflowStore = newWorkflowStore()

func newWorkflowStore() *memory.WorkflowStore {
	path := strings.TrimSpace(os.Getenv("WORKFLOW_STORE_PATH"))
	if path == "" {
		path = "data/workflow_store.json"
	}
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
	BriefModel     string                `json:"brief_model"`
	Questions      []articleQuestionJSON `json:"questions"`
}

type answerBriefSessionRequest struct {
	Content      string `json:"content"`
	SkipDeepDive bool   `json:"skip_deep_dive"`
	BriefModel   string `json:"brief_model"`
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
	TargetField      string `json:"target_field,omitempty"`
	TargetQuestionID string `json:"target_question_id,omitempty"`
	FollowUpIndex    int    `json:"follow_up_index,omitempty"`
}

type generateDraftRequest struct {
	StyleProfileID string `json:"style_profile_id"`
	SessionID      string `json:"session_id"`
	DraftModel     string `json:"draft_model"`
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

	service := newBriefInterviewService(req.BriefModel)
	result, err := service.StartSession(briefapp.StartSessionInput{
		SessionID:      req.SessionID,
		StyleProfileID: req.StyleProfileID,
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
		service := newBriefInterviewService(req.BriefModel)
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

	generator, err := llamacpp.NewClientFromEnvForPurposeWithModel("DRAFT", req.DraftModel)
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
			TargetField:      result.NextQuestion.TargetField,
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

func newBriefInterviewService(model string) *briefapp.InterviewService {
	return briefapp.NewInterviewService(llmFollowUpGenerator{model: strings.TrimSpace(model)})
}

type llmFollowUpGenerator struct {
	model string
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
	prompt := buildFollowUpPrompt(session, target, answer, followUpIndex)
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

func buildFollowUpPrompt(session briefdomain.ArticleBriefSession, target briefdomain.ArticleQuestion, answer briefdomain.BriefAnswer, followUpIndex int) string {
	return fmt.Sprintf(`あなたはNote記事の取材編集者です。
次の記事ブリーフ回答を深掘りするため、著者に聞く追加質問を1つだけ作ってください。

条件:
- 日本語で質問する
- はい/いいえで答えられる質問にしない
- 選択式にしない
- 具体的な経験、感情、判断、失敗、価値観のどれかを引き出す
- 質問文だけを出力する

セッションID: %s
対象質問: %s
回答: %s
深掘り回数: %d
`, session.ID, target.Text, answer.Content, followUpIndex)
}

func extractFollowUpQuestion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`\"'「」")
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "-*0123456789. "))
		line = strings.Trim(line, "\"'「」")
		if strings.HasSuffix(line, "？") || strings.HasSuffix(line, "?") {
			return line
		}
	}
	return value
}
