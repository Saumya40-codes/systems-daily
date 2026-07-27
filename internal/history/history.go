package history

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Entry records one delivered (or previewed) write-up.
type Entry struct {
	TopicID   string    `json:"topic_id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	SentAt    time.Time `json:"sent_at"`
	WordCount int       `json:"word_count,omitempty"`
	Subject   string    `json:"subject,omitempty"`
}

// Store is a simple append-only JSON history file.
type Store struct {
	path    string
	entries []Entry
}

type fileShape struct {
	Entries []Entry `json:"entries"`
}

// Open loads history from path (creates empty if missing).
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	var f fileShape
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	s.entries = f.Entries
	return s, nil
}

// RecentIDs returns topic IDs used within the last windowDays.
func (s *Store) RecentIDs(windowDays int) []string {
	if windowDays <= 0 {
		windowDays = 60
	}
	cutoff := time.Now().AddDate(0, 0, -windowDays)
	seen := map[string]struct{}{}
	var ids []string
	for _, e := range s.entries {
		if e.SentAt.Before(cutoff) {
			continue
		}
		if _, ok := seen[e.TopicID]; ok {
			continue
		}
		seen[e.TopicID] = struct{}{}
		ids = append(ids, e.TopicID)
	}
	return ids
}

// Append adds an entry and persists.
func (s *Store) Append(e Entry) error {
	if e.SentAt.IsZero() {
		e.SentAt = time.Now().UTC()
	}
	s.entries = append(s.entries, e)
	return s.save()
}

// All returns a copy of entries.
func (s *Store) All() []Entry {
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f := fileShape{Entries: s.entries}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
