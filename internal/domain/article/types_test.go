package article

import "testing"

func TestGenerationRequestDefaultsAndValidation(t *testing.T) {
	req := GenerationRequest{NoteURL: "https://note.com/u/n/n1", Theme: "theme"}
	req.ApplyDefaults()

	if req.StyleChoice != "ですます調" {
		t.Fatalf("unexpected style default: %q", req.StyleChoice)
	}
	if req.ToneChoice != "客観的" {
		t.Fatalf("unexpected tone default: %q", req.ToneChoice)
	}
	if req.WordCount != 1500 {
		t.Fatalf("unexpected word count default: %d", req.WordCount)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestGenerationRequestValidationErrors(t *testing.T) {
	if err := (GenerationRequest{Theme: "theme"}).Validate(); err == nil {
		t.Fatal("expected source validation error")
	}
	if err := (GenerationRequest{NoteURL: "https://note.com/u/n/n1"}).Validate(); err == nil {
		t.Fatal("expected theme validation error")
	}
}
