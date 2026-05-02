package handlers

import (
	"path/filepath"
	"testing"

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
