package memory

import (
	"fmt"
	"sync"

	"github.com/teradakousuke/note_maker/internal/application/authorstyle"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
)

// WorkflowStore is an in-memory repository for the local three-phase workflow.
type WorkflowStore struct {
	mu sync.RWMutex

	authorStyles   map[string]authorstyle.AnalyzeResult
	profileIndexes map[string]authorstyle.AnalyzeResult
	guideIndexes   map[string]authorstyle.AnalyzeResult
	sessions       map[string]briefdomain.ArticleBriefSession
	briefs         map[string]briefdomain.ArticleBrief
}

// NewWorkflowStore creates an empty local workflow store.
func NewWorkflowStore() *WorkflowStore {
	return &WorkflowStore{
		authorStyles:   make(map[string]authorstyle.AnalyzeResult),
		profileIndexes: make(map[string]authorstyle.AnalyzeResult),
		guideIndexes:   make(map[string]authorstyle.AnalyzeResult),
		sessions:       make(map[string]briefdomain.ArticleBriefSession),
		briefs:         make(map[string]briefdomain.ArticleBrief),
	}
}

// SaveAuthorStyle stores an author style analysis result.
func (s *WorkflowStore) SaveAuthorStyle(result authorstyle.AnalyzeResult) error {
	if result.ID == "" {
		return fmt.Errorf("author style result id is required")
	}
	if err := result.Profile.Validate(); err != nil {
		return err
	}
	if err := result.Guide.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.authorStyles[result.ID] = result
	s.profileIndexes[result.Profile.ID] = result
	s.guideIndexes[result.Guide.ID] = result
	return nil
}

// GetAuthorStyle returns an analysis result by analysis ID, profile ID, or guide ID.
func (s *WorkflowStore) GetAuthorStyle(id string) (authorstyle.AnalyzeResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if result, ok := s.authorStyles[id]; ok {
		return result, true
	}
	if result, ok := s.profileIndexes[id]; ok {
		return result, true
	}
	if result, ok := s.guideIndexes[id]; ok {
		return result, true
	}
	return authorstyle.AnalyzeResult{}, false
}

// SaveSession stores a brief interview session.
func (s *WorkflowStore) SaveSession(session briefdomain.ArticleBriefSession) error {
	if session.ID == "" {
		return fmt.Errorf("brief session id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return nil
}

// GetSession returns a brief interview session by ID.
func (s *WorkflowStore) GetSession(id string) (briefdomain.ArticleBriefSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	return session, ok
}

// SaveBrief stores the completed brief for a session.
func (s *WorkflowStore) SaveBrief(sessionID string, brief briefdomain.ArticleBrief) error {
	if sessionID == "" {
		return fmt.Errorf("brief session id is required")
	}
	if brief.StyleProfileID == "" {
		return fmt.Errorf("brief style profile id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.briefs[sessionID] = brief
	return nil
}

// GetBrief returns a completed brief by session ID.
func (s *WorkflowStore) GetBrief(sessionID string) (briefdomain.ArticleBrief, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	brief, ok := s.briefs[sessionID]
	return brief, ok
}

// GetProfileAndGuide returns style assets by profile, guide, or analysis ID.
func (s *WorkflowStore) GetProfileAndGuide(id string) (authordomain.AuthorStyleProfile, authordomain.WritingStyleGuide, bool) {
	result, ok := s.GetAuthorStyle(id)
	if !ok {
		return authordomain.AuthorStyleProfile{}, authordomain.WritingStyleGuide{}, false
	}
	return result.Profile, result.Guide, true
}
