package email

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestBuildMIMEPlainNoAttachment(t *testing.T) {
	raw := string(buildMIME("a@b.com", "c@d.com", "Subj", "hello\nworld", nil))
	if !strings.Contains(raw, "Content-Type: text/plain") {
		t.Fatal(raw)
	}
	if strings.Contains(raw, "multipart/") {
		t.Fatal("unexpected multipart")
	}
	if !strings.Contains(raw, "hello\r\nworld") {
		t.Fatalf("body newlines: %q", raw)
	}
}

func TestBuildMIMEWithPDFAttachment(t *testing.T) {
	pdf := []byte("%PDF-1.4 fake")
	raw := string(buildMIME("from@x", "to@y", "Daily", "body text", []Attachment{{
		Filename:    "systems-daily-2026-07-28.pdf",
		ContentType: "application/pdf",
		Data:        pdf,
	}}))
	if !strings.Contains(raw, "Content-Type: multipart/mixed; boundary=") {
		t.Fatal("missing multipart")
	}
	// Extract boundary
	i := strings.Index(raw, "boundary=")
	if i < 0 {
		t.Fatal("no boundary")
	}
	rest := raw[i+len("boundary="):]
	boundary := strings.SplitN(rest, "\r\n", 2)[0]
	if !strings.Contains(raw, "--"+boundary+"--") {
		t.Fatalf("missing closing boundary %q", boundary)
	}
	if !strings.Contains(raw, `filename="systems-daily-2026-07-28.pdf"`) {
		t.Fatal("missing filename")
	}
	if !strings.Contains(raw, "Content-Transfer-Encoding: base64") {
		t.Fatal("missing b64")
	}
	// Base64 rows <= 76 chars (excluding trailing CRLF)
	inB64 := false
	for _, line := range strings.Split(raw, "\r\n") {
		if strings.HasPrefix(line, "Content-Transfer-Encoding: base64") {
			inB64 = true
			continue
		}
		if inB64 {
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "--") {
				inB64 = false
				continue
			}
			if len(line) > 76 {
				t.Fatalf("b64 line too long %d: %q", len(line), line)
			}
		}
	}
	// Round-trip payload
	enc := base64.StdEncoding.EncodeToString(pdf)
	if !strings.Contains(strings.ReplaceAll(raw, "\r\n", ""), enc) {
		// wrapped form still contains pieces
		for i := 0; i < len(enc); i += 76 {
			end := i + 76
			if end > len(enc) {
				end = len(enc)
			}
			if !strings.Contains(raw, enc[i:end]) {
				t.Fatalf("missing b64 chunk %q", enc[i:end])
			}
		}
	}
}

func TestSanitizeHeaderStripsNewlinesAndQuotes(t *testing.T) {
	got := sanitizeHeader("a\r\nB\"x")
	if strings.Contains(got, "\n") || strings.Contains(got, "\r") {
		t.Fatalf("newlines remain: %q", got)
	}
	if strings.Contains(got, "\"") {
		t.Fatalf("quotes remain: %q", got)
	}
}

func TestWrapBase64LineLength(t *testing.T) {
	s := strings.Repeat("A", 200)
	out := wrapBase64(s)
	for _, line := range strings.Split(strings.TrimSuffix(out, "\r\n"), "\r\n") {
		if len(line) > 76 {
			t.Fatalf("line len %d", len(line))
		}
	}
}
