package handlers

import (
	"path/filepath"
	"testing"

	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
	"github.com/teradakousuke/note_maker/internal/infrastructure/repository/memory"
	sqliterepo "github.com/teradakousuke/note_maker/internal/infrastructure/repository/sqlite"
)

func TestNewWorkflowStoreCanUseSQLiteDriver(t *testing.T) {
	t.Setenv("WORKFLOW_STORE_DRIVER", "sqlite")
	t.Setenv("WORKFLOW_STORE_PATH", filepath.Join(t.TempDir(), "workflow.db"))

	store := newWorkflowStore()
	sqliteStore, ok := store.(*sqliterepo.WorkflowStore)
	if !ok {
		t.Fatalf("store type = %T, want sqlite workflow store", store)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
}

func TestNewWorkflowStoreImportsLegacyJSONWhenSQLiteIsEmpty(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "workflow_store.json")
	legacy, err := memory.NewPersistentWorkflowStore(legacyPath)
	if err != nil {
		t.Fatalf("legacy store: %v", err)
	}
	persona := personadomain.Persona{
		ID:            "custom_writer",
		DisplayName:   "Custom Writer",
		Description:   "Imported legacy persona.",
		DefaultFormat: "note_article",
		Sources:       []personadomain.AuthorSource{{Kind: "manual", Ref: "custom"}},
		VoiceNotes:    personadomain.VoiceNotes{FirstPerson: []string{"私"}, Tone: "落ち着いて書く"},
	}
	if err := legacy.SavePersona(persona); err != nil {
		t.Fatalf("save legacy persona: %v", err)
	}

	t.Setenv("WORKFLOW_STORE_DRIVER", "sqlite")
	t.Setenv("WORKFLOW_STORE_PATH", filepath.Join(dir, "workflow_store.db"))

	store := newWorkflowStore()
	sqliteStore, ok := store.(*sqliterepo.WorkflowStore)
	if !ok {
		t.Fatalf("store type = %T, want sqlite workflow store", store)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })

	imported, ok := sqliteStore.GetPersona("custom_writer")
	if !ok {
		t.Fatalf("legacy persona was not imported")
	}
	if imported.DisplayName != persona.DisplayName {
		t.Fatalf("imported persona = %#v, want %#v", imported, persona)
	}
}
