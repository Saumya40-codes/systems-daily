package topics

import (
	"math/rand"
	"os"
	"path/filepath"
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
