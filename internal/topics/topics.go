package topics

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

// Topic is a systems-ish deep-dive subject.
type Topic struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Angles   []string `json:"angles,omitempty"` // optional focus hints for the LLM
}

// Catalog is a loaded topic list.
type Catalog []Topic

type fileShape struct {
	Topics []Topic `json:"topics"`
}

// Load reads topics from path. Empty path uses the embedded default catalog.
func Load(path string) (Catalog, error) {
	var raw []byte
	var err error
	if path == "" {
		raw = defaultTopicsJSON
	} else {
		raw, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read topics file %q: %w", path, err)
		}
	}
	return parse(raw)
}

func parse(raw []byte) (Catalog, error) {
	var f fileShape
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parse topics JSON: %w", err)
	}
	if len(f.Topics) == 0 {
		return nil, fmt.Errorf("topics catalog is empty")
	}
	seen := make(map[string]struct{}, len(f.Topics))
	for i, t := range f.Topics {
		t.ID = strings.TrimSpace(t.ID)
		t.Title = strings.TrimSpace(t.Title)
		t.Category = strings.TrimSpace(t.Category)
		if t.ID == "" || t.Title == "" {
			return nil, fmt.Errorf("topic at index %d: id and title are required", i)
		}
		if _, ok := seen[t.ID]; ok {
			return nil, fmt.Errorf("duplicate topic id %q", t.ID)
		}
		seen[t.ID] = struct{}{}
		f.Topics[i] = t
	}
	return Catalog(f.Topics), nil
}

// Pick selects a topic not in recentIDs. recentIDs should be topic IDs used recently.
func (c Catalog) Pick(recentIDs []string, rng *rand.Rand) (Topic, error) {
	if len(c) == 0 {
		return Topic{}, fmt.Errorf("empty catalog")
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	blocked := make(map[string]struct{}, len(recentIDs))
	for _, id := range recentIDs {
		blocked[id] = struct{}{}
	}

	var candidates []Topic
	for _, t := range c {
		if _, ok := blocked[t.ID]; !ok {
			candidates = append(candidates, t)
		}
	}
	if len(candidates) == 0 {
		// All topics exhausted in window - fall back to full catalog
		candidates = append([]Topic(nil), c...)
	}
	return candidates[rng.Intn(len(candidates))], nil
}

// ByID looks up a topic; returns error if unknown.
func (c Catalog) ByID(id string) (Topic, error) {
	id = strings.TrimSpace(id)
	for _, t := range c {
		if t.ID == id {
			return t, nil
		}
	}
	return Topic{}, fmt.Errorf("unknown topic id %q", id)
}

// Categories returns unique category names in first-seen order.
func (c Catalog) Categories() []string {
	seen := map[string]struct{}{}
	var out []string
	for _, t := range c {
		if _, ok := seen[t.Category]; !ok {
			seen[t.Category] = struct{}{}
			out = append(out, t.Category)
		}
	}
	return out
}
