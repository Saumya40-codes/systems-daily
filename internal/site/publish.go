package site

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PublishOptions controls the static site layout on disk.
type PublishOptions struct {
	// OutDir is the web root (e.g. site/public).
	OutDir string
	// WindowDays is how many dated pages to keep (including today). Default 7.
	WindowDays int
	// Now is the page date (usually article.Generated in local/schedule TZ).
	Now time.Time
}

// Result is what was written.
type Result struct {
	DatePath  string // relative URL path e.g. /d/2026-07-28/
	TodayPath string // /today/
	Date      string // YYYY-MM-DD
	Pruned    []string
}

// Publish writes the page to d/YYYY-MM-DD/, today/, and index (→ today),
// then removes dated dirs older than the window.
func Publish(p Page, opt PublishOptions) (*Result, error) {
	if opt.OutDir == "" {
		return nil, fmt.Errorf("site out dir is empty")
	}
	if opt.WindowDays <= 0 {
		opt.WindowDays = 7
	}
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	day := opt.Now.Format("2006-01-02")

	htmlDoc, err := RenderHTML(p)
	if err != nil {
		return nil, err
	}

	dateDir := filepath.Join(opt.OutDir, "d", day)
	todayDir := filepath.Join(opt.OutDir, "today")
	if err := os.MkdirAll(dateDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(todayDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opt.OutDir, 0o755); err != nil {
		return nil, err
	}

	dateIndex := filepath.Join(dateDir, "index.html")
	todayIndex := filepath.Join(todayDir, "index.html")
	if err := os.WriteFile(dateIndex, []byte(htmlDoc), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(todayIndex, []byte(htmlDoc), 0o644); err != nil {
		return nil, err
	}

	// Root index: same content as today (works without server-side redirects).
	rootIndex := filepath.Join(opt.OutDir, "index.html")
	if err := os.WriteFile(rootIndex, []byte(htmlDoc), 0o644); err != nil {
		return nil, err
	}

	// Tiny meta for humans/tools.
	meta := fmt.Sprintf("date=%s\n", day)
	_ = os.WriteFile(filepath.Join(opt.OutDir, "CURRENT"), []byte(meta), 0o644)

	pruned, err := pruneOld(opt.OutDir, opt.Now, opt.WindowDays)
	if err != nil {
		return nil, err
	}

	return &Result{
		DatePath:  "/d/" + day + "/",
		TodayPath: "/today/",
		Date:      day,
		Pruned:    pruned,
	}, nil
}

func pruneOld(outDir string, now time.Time, windowDays int) ([]string, error) {
	dRoot := filepath.Join(outDir, "d")
	entries, err := os.ReadDir(dRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	cutoff := now.AddDate(0, 0, -(windowDays - 1))
	// Keep if date >= cutoff (start of that day).
	cutoffDay := time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, now.Location())

	var pruned []string
	var kept []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		day, err := time.ParseInLocation("2006-01-02", name, now.Location())
		if err != nil {
			continue // ignore non-date dirs
		}
		if day.Before(cutoffDay) {
			path := filepath.Join(dRoot, name)
			if err := os.RemoveAll(path); err != nil {
				return pruned, err
			}
			pruned = append(pruned, name)
		} else {
			kept = append(kept, name)
		}
	}
	sort.Strings(pruned)
	sort.Strings(kept)
	return pruned, nil
}

// PublicURL joins base URL with a path like /today/.
func PublicURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if path == "" {
		return base
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if base == "" {
		return path
	}
	return base + path
}
