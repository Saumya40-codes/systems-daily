package topics

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadDefault(t *testing.T) Catalog {
	t.Helper()
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPickAvoidsRecent(t *testing.T) {
	c := loadDefault(t)
	recent := make([]string, 0, len(c)-1)
	for i := 0; i < len(c)-1; i++ {
		recent = append(recent, c[i].ID)
	}
	rng := rand.New(rand.NewSource(1))
	got, err := c.Pick(recent, rng)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != c[len(c)-1].ID {
		t.Fatalf("expected last free topic, got %s", got.ID)
	}
}

func TestPickFallbackWhenAllRecent(t *testing.T) {
	c := loadDefault(t)
	recent := make([]string, len(c))
	for i, tpc := range c {
		recent[i] = tpc.ID
	}
	rng := rand.New(rand.NewSource(42))
	got, err := c.Pick(recent, rng)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "" {
		t.Fatal("empty topic")
	}
}

func TestByID(t *testing.T) {
	c := loadDefault(t)
	if _, err := c.ByID("buddy-allocator"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ByID("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCatalogIDsUniqueAndValid(t *testing.T) {
	c := loadDefault(t)
	if len(c) < 10 {
		t.Fatalf("expected a real catalog, got %d", len(c))
	}
	seen := map[string]struct{}{}
	for _, tp := range c {
		if tp.ID == "" || tp.Title == "" {
			t.Fatalf("empty id/title: %+v", tp)
		}
		if _, ok := seen[tp.ID]; ok {
			t.Fatalf("duplicate id %s", tp.ID)
		}
		seen[tp.ID] = struct{}{}
	}
}

func TestDefaultCatalogHasResearchReadyDatabaseAndProductionTopics(t *testing.T) {
	c := loadDefault(t)
	ready, databases, namedProduction, planetScale := 0, 0, false, false
	for _, tp := range c {
		if !tp.ResearchReady() {
			continue
		}
		ready++
		if tp.Category == "database" {
			databases++
		}
		for _, source := range tp.Sources {
			if strings.Contains(source.Title, "Cloudflare") {
				namedProduction = true
			}
			if strings.Contains(source.Title, "PlanetScale") {
				planetScale = true
			}
		}
	}
	if ready < 7 || databases < 6 || !namedProduction || !planetScale {
		t.Fatalf("ready=%d databases=%d namedProduction=%v planetScale=%v", ready, databases, namedProduction, planetScale)
	}
}

func TestPickResearchReadyDoesNotRepeatExhaustedPool(t *testing.T) {
	c := Catalog{{ID: "ready", Title: "Ready", CoreQuestion: "Why?", Sources: []Source{{ID: "doc"}}}}
	if _, err := c.PickResearchReady([]string{"ready"}, rand.New(rand.NewSource(1))); err == nil {
		t.Fatal("expected exhausted-pool error")
	}
}

func TestPickResearchReadyKeepsSeventhDailySlot(t *testing.T) {
	var catalog Catalog
	var recent []string
	for i := 0; i < 7; i++ {
		id := fmt.Sprintf("ready-%d", i)
		catalog = append(catalog, Topic{ID: id, Title: id, CoreQuestion: "Why?", Sources: []Source{{ID: "doc"}}})
		if i < 6 {
			recent = append(recent, id)
		}
	}
	got, err := catalog.PickResearchReady(recent, rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "ready-6" {
		t.Fatalf("got %s", got.ID)
	}
}

func TestPickResearchReadySkipsBacklog(t *testing.T) {
	c := Catalog{
		{ID: "backlog", Title: "Backlog"},
		{ID: "ready", Title: "Ready", CoreQuestion: "Why?", Sources: []Source{{ID: "doc"}}},
	}
	got, err := c.PickResearchReady(nil, rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "ready" {
		t.Fatalf("got %s", got.ID)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "topics.json")
	body := `{
  "topics": [
    {
      "id": "custom-one",
      "title": "Custom topic",
      "category": "custom",
      "angles": ["a", "b"]
    }
  ]
}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c) != 1 || c[0].ID != "custom-one" {
		t.Fatalf("got %+v", c)
	}
}

func TestLoadRejectsDuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	body := `{"topics":[
		{"id":"x","title":"One","category":"c"},
		{"id":"x","title":"Two","category":"c"}
	]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestLoadRejectsUnsafeSourceURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad-source.json")
	body := `{"topics":[{"id":"x","title":"X","core_question":"Why?","sources":[{"id":"doc","title":"Doc","url":"javascript:alert(1)","evidence":"fact"}]}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unsafe URL error")
	}
}

func TestLoadRejectsUnsafeTopicID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad-id.json")
	if err := os.WriteFile(path, []byte(`{"topics":[{"id":"../escape","title":"X"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected unsafe topic ID error")
	}
}

func TestLoadRejectsOversizedEvidence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large-source.json")
	body := `{"topics":[{"id":"x","title":"X","sources":[{"id":"doc","title":"Doc","url":"https://example.com","evidence":"` + strings.Repeat("x", maxEvidenceBytes+1) + `"}]}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected evidence size error")
	}
}
