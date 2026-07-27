package topics

import (
	"math/rand"
	"testing"
)

func TestPickAvoidsRecent(t *testing.T) {
	recent := make([]string, 0, len(Catalog)-1)
	for i := 0; i < len(Catalog)-1; i++ {
		recent = append(recent, Catalog[i].ID)
	}
	rng := rand.New(rand.NewSource(1))
	got, err := Pick(recent, rng)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != Catalog[len(Catalog)-1].ID {
		// With only one free topic it must pick that one
		t.Fatalf("expected last free topic, got %s", got.ID)
	}
}

func TestPickFallbackWhenAllRecent(t *testing.T) {
	recent := make([]string, len(Catalog))
	for i, tpc := range Catalog {
		recent[i] = tpc.ID
	}
	rng := rand.New(rand.NewSource(42))
	got, err := Pick(recent, rng)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "" {
		t.Fatal("empty topic")
	}
}

func TestByID(t *testing.T) {
	_, err := ByID("buddy-allocator")
	if err != nil {
		t.Fatal(err)
	}
	_, err = ByID("nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCatalogIDsUnique(t *testing.T) {
	seen := map[string]struct{}{}
	for _, tp := range Catalog {
		if tp.ID == "" || tp.Title == "" {
			t.Fatalf("empty id/title: %+v", tp)
		}
		if _, ok := seen[tp.ID]; ok {
			t.Fatalf("duplicate id %s", tp.ID)
		}
		seen[tp.ID] = struct{}{}
	}
}
