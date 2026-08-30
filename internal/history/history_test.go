package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHistoryProvenanceRoundTripAndOldFileCompatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, []byte(`{"entries":[{"topic_id":"old","title":"Old","category":"os","sent_at":"2026-01-01T00:00:00Z"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append(Entry{TopicID: "new", Pipeline: "grounded-v1", Model: "model", SourceIDs: []string{"doc"}, Artifact: "data/articles/a.json"}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	entries := reopened.All()
	if len(entries) != 2 || entries[0].Pipeline != "" || entries[1].Pipeline != "grounded-v1" || entries[1].SourceIDs[0] != "doc" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}
