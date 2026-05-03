package persona

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	IDTerisuke = "terisuke"
	IDCloudia  = "cloudia"
)

var customIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)

// AuthorSource identifies public material used to derive a persona's style.
type AuthorSource struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
	URL  string `json:"url"`
}

// VoiceNotes are practical prompt hints for one writing identity.
type VoiceNotes struct {
	FirstPerson   []string `json:"first_person"`
	Tone          string   `json:"tone"`
	TitlePatterns []string `json:"title_patterns"`
	AntiPatterns  []string `json:"anti_patterns"`
}

// Persona is a selectable writing identity.
type Persona struct {
	ID            string         `json:"id"`
	DisplayName   string         `json:"display_name"`
	Description   string         `json:"description"`
	DefaultFormat string         `json:"default_format"`
	Sources       []AuthorSource `json:"sources"`
	VoiceNotes    VoiceNotes     `json:"voice_notes"`
}

// ValidateCustom checks the user-authored persona fields needed by prompt and UI flows.
func (p Persona) ValidateCustom() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("persona id is required")
	}
	if !customIDPattern.MatchString(strings.TrimSpace(p.ID)) {
		return fmt.Errorf("persona id must use 2-64 lowercase letters, digits, hyphen, or underscore")
	}
	if strings.TrimSpace(p.DisplayName) == "" {
		return fmt.Errorf("persona display name is required")
	}
	if strings.TrimSpace(p.DefaultFormat) == "" {
		return fmt.Errorf("persona default format is required")
	}
	for i, source := range p.Sources {
		if strings.TrimSpace(source.Kind) == "" {
			return fmt.Errorf("persona source %d kind is required", i)
		}
		if strings.TrimSpace(source.Ref) == "" && strings.TrimSpace(source.URL) == "" {
			return fmt.Errorf("persona source %d requires ref or url", i)
		}
	}
	return nil
}

// PromptHint turns voice notes into concise draft-generation guidance.
func (p Persona) PromptHint() string {
	var lines []string
	if p.DisplayName != "" {
		lines = append(lines, "人格: "+p.DisplayName)
	}
	if p.VoiceNotes.Tone != "" {
		lines = append(lines, "トーン: "+p.VoiceNotes.Tone)
	}
	if len(p.VoiceNotes.FirstPerson) > 0 {
		lines = append(lines, "一人称候補: "+strings.Join(p.VoiceNotes.FirstPerson, " / "))
	}
	if len(p.VoiceNotes.TitlePatterns) > 0 {
		lines = append(lines, "タイトル傾向: "+strings.Join(p.VoiceNotes.TitlePatterns, "、"))
	}
	if len(p.VoiceNotes.AntiPatterns) > 0 {
		lines = append(lines, "避ける混線: "+strings.Join(p.VoiceNotes.AntiPatterns, "、"))
	}
	return strings.Join(lines, "\n")
}

// Registry resolves personas by id.
type Registry struct {
	personas map[string]Persona
	order    []string
}

// DefaultRegistry returns the built-in persona set.
func DefaultRegistry() Registry {
	personas := []Persona{
		Terisuke(),
		Cloudia(),
	}
	registry := Registry{personas: make(map[string]Persona, len(personas)), order: make([]string, 0, len(personas))}
	for _, item := range personas {
		registry.personas[item.ID] = item
		registry.order = append(registry.order, item.ID)
	}
	return registry
}

// List returns personas in stable UI order.
func (r Registry) List() []Persona {
	result := make([]Persona, 0, len(r.order))
	for _, id := range r.order {
		if item, ok := r.personas[id]; ok {
			result = append(result, item)
		}
	}
	return result
}

// Get returns a persona, defaulting empty ids to terisuke.
func (r Registry) Get(id string) (Persona, bool) {
	id = NormalizeID(id)
	item, ok := r.personas[id]
	return item, ok
}

// NormalizeID returns the default persona when id is empty.
func NormalizeID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return IDTerisuke
	}
	return id
}
