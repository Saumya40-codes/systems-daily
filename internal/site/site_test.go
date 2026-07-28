package site

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenderHTMLFromMarkdown(t *testing.T) {
	htmlDoc, err := RenderHTML(Page{
		Title:    "Windowed WDT",
		Category: "embedded",
		Date:     time.Date(2026, 7, 28, 8, 0, 0, 0, time.UTC),
		BodyMarkdown: `# Windowed WDT

A short note.

` + "```text\n  A --> B\n```" + `
`,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<!DOCTYPE html>", "systems-daily", "embedded", "Content-Security-Policy", "<pre", "theme-toggle", "data-theme", "script-src 'unsafe-inline'"} {
		if !strings.Contains(htmlDoc, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestRenderHTMLFromFragment(t *testing.T) {
	htmlDoc, err := RenderHTML(Page{
		Title:    "",
		Category: "os",
		Date:     time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC),
		BodyMarkdown: `<h1>FUTEX_WAIT slow path</h1>
<p>When CAS fails, the thread enters the kernel.</p>
<pre>val == expected</pre>
<script>alert(1)</script>
<p onclick="x()">safe?</p>`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(htmlDoc, "FUTEX_WAIT slow path") {
		t.Fatal("title/body missing")
	}
	// Model-supplied script/handlers stripped; shell may keep a tiny theme script.
	if strings.Contains(htmlDoc, "alert") {
		t.Fatal("model script should be stripped")
	}
	if strings.Contains(htmlDoc, "onclick") {
		t.Fatal("onclick should be stripped")
	}
	if !strings.Contains(htmlDoc, "theme-toggle") {
		t.Fatal("expected theme toggle in shell")
	}
	if !strings.Contains(htmlDoc, "<pre>val == expected</pre>") {
		t.Fatal("pre should remain")
	}
}

func TestLooksLikeHTML(t *testing.T) {
	if !looksLikeHTML("<h1>x</h1><p>y</p>") {
		t.Fatal("expected html")
	}
	if looksLikeHTML("# markdown\n\npara") {
		t.Fatal("expected markdown")
	}
}

func TestPublishAndPrune(t *testing.T) {
	root := t.TempDir()
	loc := time.UTC
	for _, d := range []string{"2026-07-20", "2026-07-21", "2026-07-25"} {
		dir := filepath.Join(root, "d", d)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 7, 28, 9, 0, 0, 0, loc)
	res, err := Publish(Page{
		Title:        "Today note",
		Category:     "os",
		Date:         now,
		BodyMarkdown: "# Today note\n\nHello.\n",
	}, PublishOptions{OutDir: root, WindowDays: 7, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if res.Date != "2026-07-28" {
		t.Fatalf("date %s", res.Date)
	}
	if _, err := os.Stat(filepath.Join(root, "d", "2026-07-20", "index.html")); !os.IsNotExist(err) {
		t.Fatal("expected prune 20")
	}
	if _, err := os.Stat(filepath.Join(root, "today", "index.html")); err != nil {
		t.Fatal(err)
	}
}

func TestPublicURL(t *testing.T) {
	if g := PublicURL("https://x.vercel.app/", "/today/"); g != "https://x.vercel.app/today/" {
		t.Fatal(g)
	}
}

func TestTitleFromBodyHTML(t *testing.T) {
	got := TitleFromBody("<h1>Hello <em>there</em></h1><p>x</p>", "fb")
	if got != "Hello there" {
		t.Fatalf("got %q", got)
	}
}

func TestFilterUselessSVG(t *testing.T) {
	stub := `<p>x</p><svg width="200" height="100"><rect x="10" y="10" width="50" height="50" fill="#ccc"/><text x="70" y="30">WDT</text><text x="70" y="50">Window</text></svg><p>y</p>`
	got := sanitizeFragment(stub)
	if strings.Contains(got, "<svg") {
		t.Fatalf("stub svg should drop: %s", got)
	}
	good := `<svg viewBox="0 0 200 40"><rect x="0" y="10" width="40" height="20"/><line x1="40" y1="20" x2="80" y2="20"/><rect x="80" y="10" width="40" height="20"/><text x="20" y="25">open</text><text x="100" y="25">closed</text></svg>`
	if !isUsefulSVG(good) {
		t.Fatal("expected useful")
	}
}
