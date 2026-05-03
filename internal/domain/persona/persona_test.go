package persona

import (
	"testing"

	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
)

func TestDefaultRegistryContainsSeedPersonas(t *testing.T) {
	registry := DefaultRegistry()
	for _, id := range []string{IDTerisuke, IDCloudia} {
		item, ok := registry.Get(id)
		if !ok {
			t.Fatalf("missing persona %s", id)
		}
		if item.DefaultFormat == "" || len(item.Sources) == 0 || item.PromptHint() == "" {
			t.Fatalf("persona is incomplete: %#v", item)
		}
	}
}

func TestPersonasKeepVoicesSeparate(t *testing.T) {
	terisuke, _ := DefaultRegistry().Get(IDTerisuke)
	cloudia, _ := DefaultRegistry().Get(IDCloudia)
	if terisuke.DefaultFormat == cloudia.DefaultFormat {
		t.Fatal("seed personas should default to different formats")
	}
	if terisuke.PromptHint() == cloudia.PromptHint() {
		t.Fatal("seed personas should have distinct prompt hints")
	}
}

func TestSeedPersonasReferenceRegisteredFormatsAndExpectedSources(t *testing.T) {
	registry := DefaultRegistry()
	tests := []struct {
		personaID       string
		defaultFormatID string
		sourceKinds     []string
		firstPerson     string
	}{
		{
			personaID:       IDTerisuke,
			defaultFormatID: outputformat.IDNoteArticle,
			sourceKinds:     []string{"note", "rss", "github"},
			firstPerson:     "僕",
		},
		{
			personaID:       IDCloudia,
			defaultFormatID: outputformat.IDZennArticle,
			sourceKinds:     []string{"zenn", "qiita"},
			firstPerson:     "クラウディア",
		},
	}

	for _, tt := range tests {
		t.Run(tt.personaID, func(t *testing.T) {
			item, ok := registry.Get(tt.personaID)
			if !ok {
				t.Fatalf("missing persona %s", tt.personaID)
			}
			if item.DefaultFormat != tt.defaultFormatID {
				t.Fatalf("default format = %s, want %s", item.DefaultFormat, tt.defaultFormatID)
			}
			if _, ok := outputformat.DefaultRegistry().Get(item.DefaultFormat); !ok {
				t.Fatalf("persona %s references unregistered format %s", item.ID, item.DefaultFormat)
			}
			for _, wantKind := range tt.sourceKinds {
				if !hasSourceKind(item, wantKind) {
					t.Fatalf("persona %s missing source kind %s", item.ID, wantKind)
				}
			}
			if !contains(item.VoiceNotes.FirstPerson, tt.firstPerson) {
				t.Fatalf("persona %s missing first person %s", item.ID, tt.firstPerson)
			}
		})
	}
}

func hasSourceKind(item Persona, kind string) bool {
	for _, source := range item.Sources {
		if source.Kind == kind {
			return true
		}
	}
	return false
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
