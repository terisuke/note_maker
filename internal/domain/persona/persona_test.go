package persona

import "testing"

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
