package app

import (
	"testing"

	"github.com/Saumya40-codes/systems-daily/internal/topics"
)

func TestCitedSourcesOmitsUnusedBibliographyEntries(t *testing.T) {
	all := []topics.Source{{ID: "one"}, {ID: "two"}, {ID: "three"}}
	got := citedSources(all, []string{"three", "one"})
	if len(got) != 2 || got[0].ID != "one" || got[1].ID != "three" {
		t.Fatalf("got %+v", got)
	}
}
