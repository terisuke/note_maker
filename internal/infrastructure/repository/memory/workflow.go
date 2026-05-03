package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/teradakousuke/note_maker/internal/application/authorstyle"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

// WorkflowStore is an in-memory repository for the local three-phase workflow.
type WorkflowStore struct {
	mu   sync.RWMutex
	path string

	authorStyles   map[string]authorstyle.AnalyzeResult
	profileIndexes map[string]authorstyle.AnalyzeResult
	guideIndexes   map[string]authorstyle.AnalyzeResult
	sessions       map[string]briefdomain.ArticleBriefSession
	briefs         map[string]briefdomain.ArticleBrief
	briefVersions  map[string][]briefdomain.ArticleBriefVersion
	personas       map[string]personadomain.Persona
}

type workflowSnapshot struct {
	AuthorStyles  map[string]authorstyle.AnalyzeResult         `json:"author_styles"`
	Sessions      map[string]briefdomain.ArticleBriefSession   `json:"sessions"`
	Briefs        map[string]briefdomain.ArticleBrief          `json:"briefs"`
	BriefVersions map[string][]briefdomain.ArticleBriefVersion `json:"brief_versions,omitempty"`
	Personas      map[string]personadomain.Persona             `json:"personas,omitempty"`
}

// NewWorkflowStore creates an empty local workflow store.
func NewWorkflowStore() *WorkflowStore {
	return &WorkflowStore{
		authorStyles:   make(map[string]authorstyle.AnalyzeResult),
		profileIndexes: make(map[string]authorstyle.AnalyzeResult),
		guideIndexes:   make(map[string]authorstyle.AnalyzeResult),
		sessions:       make(map[string]briefdomain.ArticleBriefSession),
		briefs:         make(map[string]briefdomain.ArticleBrief),
		briefVersions:  make(map[string][]briefdomain.ArticleBriefVersion),
		personas:       make(map[string]personadomain.Persona),
	}
}

// NewPersistentWorkflowStore creates a workflow store backed by a JSON file.
func NewPersistentWorkflowStore(path string) (*WorkflowStore, error) {
	if path == "" {
		return nil, fmt.Errorf("workflow store path is required")
	}
	store := NewWorkflowStore()
	store.path = path
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
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
	return s.persistLocked()
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

// ListAuthorStyles returns all stored author style analyses.
func (s *WorkflowStore) ListAuthorStyles() ([]authorstyle.AnalyzeResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	results := make([]authorstyle.AnalyzeResult, 0, len(s.authorStyles))
	for _, result := range s.authorStyles {
		results = append(results, result)
	}
	return results, nil
}

// SaveSession stores a brief interview session.
func (s *WorkflowStore) SaveSession(session briefdomain.ArticleBriefSession) error {
	if session.ID == "" {
		return fmt.Errorf("brief session id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return s.persistLocked()
}

// GetSession returns a brief interview session by ID.
func (s *WorkflowStore) GetSession(id string) (briefdomain.ArticleBriefSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	return session, ok
}

// ListSessions returns all stored brief interview sessions.
func (s *WorkflowStore) ListSessions() ([]briefdomain.ArticleBriefSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessions := make([]briefdomain.ArticleBriefSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	return sessions, nil
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
	s.appendBriefVersionLocked(sessionID, brief)
	s.briefs[sessionID] = brief
	return s.persistLocked()
}

// GetBrief returns a completed brief by session ID.
func (s *WorkflowStore) GetBrief(sessionID string) (briefdomain.ArticleBrief, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	brief, ok := s.briefs[sessionID]
	return brief, ok
}

// ListBriefs returns all stored completed briefs by session id.
func (s *WorkflowStore) ListBriefs() (map[string]briefdomain.ArticleBrief, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	briefs := make(map[string]briefdomain.ArticleBrief, len(s.briefs))
	for sessionID, brief := range s.briefs {
		briefs[sessionID] = brief
	}
	return briefs, nil
}

// ListBriefVersions returns persisted brief revisions for a session.
func (s *WorkflowStore) ListBriefVersions(sessionID string) ([]briefdomain.ArticleBriefVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions := s.briefVersions[sessionID]
	if len(versions) == 0 {
		brief, ok := s.briefs[sessionID]
		if !ok {
			return []briefdomain.ArticleBriefVersion{}, nil
		}
		return []briefdomain.ArticleBriefVersion{{
			SessionID: sessionID,
			Version:   1,
			Brief:     brief,
		}}, nil
	}
	result := make([]briefdomain.ArticleBriefVersion, len(versions))
	copy(result, versions)
	return result, nil
}

// SavePersona stores a user-authored persona.
func (s *WorkflowStore) SavePersona(persona personadomain.Persona) error {
	if err := persona.ValidateCustom(); err != nil {
		return err
	}
	if persona.ID == "" {
		return fmt.Errorf("persona id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.personas[persona.ID] = persona
	return s.persistLocked()
}

// GetPersona returns a user-authored persona by ID.
func (s *WorkflowStore) GetPersona(id string) (personadomain.Persona, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	persona, ok := s.personas[id]
	return persona, ok
}

// ListPersonas returns all user-authored personas.
func (s *WorkflowStore) ListPersonas() ([]personadomain.Persona, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	personas := make([]personadomain.Persona, 0, len(s.personas))
	for _, persona := range s.personas {
		personas = append(personas, persona)
	}
	return personas, nil
}

// DeletePersona removes an unreferenced user-authored persona.
func (s *WorkflowStore) DeletePersona(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = personadomain.NormalizeID(id)
	if _, ok := s.personas[id]; !ok {
		return personadomain.ErrPersonaNotFound
	}
	if s.personaReferencedLocked(id) {
		return personadomain.ErrPersonaReferenced
	}
	delete(s.personas, id)
	return s.persistLocked()
}

// GetProfileAndGuide returns style assets by profile, guide, or analysis ID.
func (s *WorkflowStore) GetProfileAndGuide(id string) (authordomain.AuthorStyleProfile, authordomain.WritingStyleGuide, bool) {
	result, ok := s.GetAuthorStyle(id)
	if !ok {
		return authordomain.AuthorStyleProfile{}, authordomain.WritingStyleGuide{}, false
	}
	return result.Profile, result.Guide, true
}

func (s *WorkflowStore) load() error {
	encoded, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read workflow store: %w", err)
	}
	var snapshot workflowSnapshot
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return fmt.Errorf("decode workflow store: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authorStyles = nonNilAuthorStyles(snapshot.AuthorStyles)
	s.sessions = nonNilSessions(snapshot.Sessions)
	s.briefs = nonNilBriefs(snapshot.Briefs)
	s.briefVersions = nonNilBriefVersions(snapshot.BriefVersions)
	s.personas = nonNilPersonas(snapshot.Personas)
	s.rebuildIndexesLocked()
	return nil
}

func (s *WorkflowStore) persistLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create workflow store dir: %w", err)
	}
	snapshot := workflowSnapshot{
		AuthorStyles:  s.authorStyles,
		Sessions:      s.sessions,
		Briefs:        s.briefs,
		BriefVersions: s.briefVersions,
		Personas:      s.personas,
	}
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workflow store: %w", err)
	}
	tempPath := s.path + ".tmp"
	if err := os.WriteFile(tempPath, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write workflow store temp: %w", err)
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		return fmt.Errorf("replace workflow store: %w", err)
	}
	return nil
}

func (s *WorkflowStore) rebuildIndexesLocked() {
	s.profileIndexes = make(map[string]authorstyle.AnalyzeResult, len(s.authorStyles))
	s.guideIndexes = make(map[string]authorstyle.AnalyzeResult, len(s.authorStyles))
	for id, result := range s.authorStyles {
		if result.ID == "" {
			result.ID = id
		}
		if result.Profile.ID != "" {
			s.profileIndexes[result.Profile.ID] = result
		}
		if result.Guide.ID != "" {
			s.guideIndexes[result.Guide.ID] = result
		}
	}
}

func (s *WorkflowStore) appendBriefVersionLocked(sessionID string, brief briefdomain.ArticleBrief) {
	versions := s.briefVersions[sessionID]
	if len(versions) == 0 {
		if previous, ok := s.briefs[sessionID]; ok {
			versions = append(versions, briefdomain.ArticleBriefVersion{
				SessionID: sessionID,
				Version:   1,
				Brief:     previous,
				CreatedAt: time.Now().UTC(),
			})
		}
	}
	versions = append(versions, briefdomain.ArticleBriefVersion{
		SessionID: sessionID,
		Version:   len(versions) + 1,
		Brief:     brief,
		CreatedAt: time.Now().UTC(),
	})
	s.briefVersions[sessionID] = versions
}

func (s *WorkflowStore) personaReferencedLocked(id string) bool {
	for _, session := range s.sessions {
		if session.PersonaID == id {
			return true
		}
	}
	for _, brief := range s.briefs {
		if brief.PersonaID == id {
			return true
		}
	}
	for _, versions := range s.briefVersions {
		for _, version := range versions {
			if version.Brief.PersonaID == id {
				return true
			}
		}
	}
	return false
}

func nonNilAuthorStyles(values map[string]authorstyle.AnalyzeResult) map[string]authorstyle.AnalyzeResult {
	if values == nil {
		return make(map[string]authorstyle.AnalyzeResult)
	}
	return values
}

func nonNilSessions(values map[string]briefdomain.ArticleBriefSession) map[string]briefdomain.ArticleBriefSession {
	if values == nil {
		return make(map[string]briefdomain.ArticleBriefSession)
	}
	return values
}

func nonNilBriefs(values map[string]briefdomain.ArticleBrief) map[string]briefdomain.ArticleBrief {
	if values == nil {
		return make(map[string]briefdomain.ArticleBrief)
	}
	return values
}

func nonNilBriefVersions(values map[string][]briefdomain.ArticleBriefVersion) map[string][]briefdomain.ArticleBriefVersion {
	if values == nil {
		return make(map[string][]briefdomain.ArticleBriefVersion)
	}
	return values
}

func nonNilPersonas(values map[string]personadomain.Persona) map[string]personadomain.Persona {
	if values == nil {
		return make(map[string]personadomain.Persona)
	}
	return values
}
