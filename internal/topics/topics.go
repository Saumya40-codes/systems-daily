package topics

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var catalogIDRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

const (
	maxSourcesPerTopic = 8
	maxEvidenceBytes   = 4000
	maxTopicEvidence   = 16000
)

// Source is curated evidence supplied to the writer. Evidence is a factual
// summary, not executable instructions or model-generated research.
type Source struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Published string `json:"published,omitempty"`
	Evidence  string `json:"evidence"`
}

// Topic is a systems-ish deep-dive subject.
type Topic struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Category     string   `json:"category"`
	CoreQuestion string   `json:"core_question,omitempty"`
	Angles       []string `json:"angles,omitempty"`
	Sources      []Source `json:"sources,omitempty"`
}

// ResearchReady reports whether a topic has enough curated material for the
// grounded editorial pipeline.
func (t Topic) ResearchReady() bool {
	return strings.TrimSpace(t.CoreQuestion) != "" && len(t.Sources) > 0
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
		if !catalogIDRE.MatchString(t.ID) {
			return nil, fmt.Errorf("topic at index %d: invalid id %q", i, t.ID)
		}
		if _, ok := seen[t.ID]; ok {
			return nil, fmt.Errorf("duplicate topic id %q", t.ID)
		}
		sourceIDs := make(map[string]struct{}, len(t.Sources))
		if len(t.Sources) > maxSourcesPerTopic {
			return nil, fmt.Errorf("topic %q: at most %d sources are allowed", t.ID, maxSourcesPerTopic)
		}
		totalEvidence := 0
		for j := range t.Sources {
			s := &t.Sources[j]
			s.ID = strings.TrimSpace(s.ID)
			s.Title = strings.TrimSpace(s.Title)
			s.URL = strings.TrimSpace(s.URL)
			s.Evidence = strings.TrimSpace(s.Evidence)
			if !catalogIDRE.MatchString(s.ID) {
				return nil, fmt.Errorf("topic %q source %d: invalid id %q", t.ID, j, s.ID)
			}
			if s.Title == "" || s.Evidence == "" {
				return nil, fmt.Errorf("topic %q source %q: title and evidence are required", t.ID, s.ID)
			}
			if len(s.Evidence) > maxEvidenceBytes {
				return nil, fmt.Errorf("topic %q source %q: evidence exceeds %d bytes", t.ID, s.ID, maxEvidenceBytes)
			}
			totalEvidence += len(s.Evidence)
			u, err := url.Parse(s.URL)
			if err != nil || u.Scheme != "https" || u.Host == "" {
				return nil, fmt.Errorf("topic %q source %q: URL must be absolute HTTPS", t.ID, s.ID)
			}
			if _, ok := sourceIDs[s.ID]; ok {
				return nil, fmt.Errorf("topic %q: duplicate source id %q", t.ID, s.ID)
			}
			sourceIDs[s.ID] = struct{}{}
		}
		if totalEvidence > maxTopicEvidence {
			return nil, fmt.Errorf("topic %q: source evidence exceeds %d bytes", t.ID, maxTopicEvidence)
		}
		seen[t.ID] = struct{}{}
		f.Topics[i] = t
	}
	return Catalog(f.Topics), nil
}

// PickResearchReady selects a sourced topic not used recently. Unsourced
// catalog entries remain available as an editorial backlog but are not sent to
// the production generator.
func (c Catalog) PickResearchReady(recentIDs []string, rng *rand.Rand) (Topic, error) {
	blocked := make(map[string]struct{}, len(recentIDs))
	for _, id := range recentIDs {
		blocked[id] = struct{}{}
	}
	ready := make(Catalog, 0, len(c))
	for _, t := range c {
		if _, used := blocked[t.ID]; t.ResearchReady() && !used {
			ready = append(ready, t)
		}
	}
	if len(ready) == 0 {
		return Topic{}, fmt.Errorf("no unused research-ready topics; add sourced topics or shorten the history window")
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return ready[rng.Intn(len(ready))], nil
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
