package pdfdoc

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Saumya40-codes/systems-daily/internal/diagrams"
	"github.com/go-pdf/fpdf"
)

// Input is everything needed to build one daily PDF.
type Input struct {
	Title    string
	Category string
	Body     string // markdown-ish with {{figure:N}} markers
	Figures  []diagrams.Figure
	Date     time.Time
}

// Build renders a readable A4 PDF from the article markdown.
func Build(in Input) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(18, 18, 18)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AliasNbPages("")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 8, fmt.Sprintf("systems-daily  |  %d / {nb}", pdf.PageNo()),
			"", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})
	pdf.AddPage()

	// Title block
	pdf.SetFont("Helvetica", "B", 16)
	title := firstHeadingOr(in.Title, in.Body)
	pdf.MultiCell(0, 8, sanitizePDF(title), "", "", false)
	pdf.Ln(2)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(90, 90, 90)
	// ASCII-only separators: core Helvetica cannot reliably draw middle-dots etc.
	meta := strings.TrimSpace(fmt.Sprintf("%s  |  %s",
		in.Category, in.Date.Format("2006-01-02")))
	if in.Category != "" || !in.Date.IsZero() {
		pdf.CellFormat(0, 5, sanitizePDF(meta), "", 1, "L", false, 0, "")
	}
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(3)
	// Rule
	w, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(left, pdf.GetY(), w-right, pdf.GetY())
	pdf.Ln(6)

	if err := writeMarkdown(pdf, in.Body, in.Figures); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("write pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func firstHeadingOr(fallback, body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
		if strings.HasPrefix(line, "## ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "## "))
		}
	}
	if fallback != "" {
		return fallback
	}
	return "Systems daily"
}

func writeMarkdown(pdf *fpdf.Fpdf, body string, figures []diagrams.Figure) error {
	lines := strings.Split(body, "\n")
	i := 0
	for i < len(lines) {
		line := lines[i]
		trim := strings.TrimSpace(line)

		// Figure marker
		if idx, ok := diagrams.ParseFigureMarker(trim); ok {
			if idx >= 0 && idx < len(figures) && len(figures[idx].PNG) > 0 {
				if err := writeFigure(pdf, figures[idx]); err != nil {
					return err
				}
			}
			i++
			continue
		}

		// Fenced code block (also the preferred home for ASCII diagrams)
		if strings.HasPrefix(trim, "```") {
			lang := strings.TrimSpace(strings.TrimPrefix(trim, "```"))
			var code []string
			i++
			for i < len(lines) {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					i++
					break
				}
				// Preserve leading spaces / empty lines for diagrams and code.
				code = append(code, lines[i])
				i++
			}
			writeCodeBlock(pdf, lang, strings.Join(code, "\n"))
			continue
		}

		// Blank line
		if trim == "" {
			pdf.Ln(3)
			i++
			continue
		}

		// Headings
		if strings.HasPrefix(trim, "### ") {
			pdf.Ln(2)
			pdf.SetFont("Helvetica", "B", 12)
			pdf.MultiCell(0, 6, sanitizePDF(strings.TrimPrefix(trim, "### ")), "", "", false)
			pdf.Ln(1)
			i++
			continue
		}
		if strings.HasPrefix(trim, "## ") {
			pdf.Ln(3)
			pdf.SetFont("Helvetica", "B", 13)
			pdf.MultiCell(0, 7, sanitizePDF(strings.TrimPrefix(trim, "## ")), "", "", false)
			pdf.Ln(1)
			i++
			continue
		}
		if strings.HasPrefix(trim, "# ") {
			// Skip duplicate top-level title already printed in header
			i++
			continue
		}

		// Unordered list
		if strings.HasPrefix(trim, "- ") || strings.HasPrefix(trim, "* ") {
			item := trim[2:]
			pdf.SetFont("Helvetica", "", 10)
			pdf.CellFormat(6, 5, "-", "", 0, "L", false, 0, "")
			pdf.MultiCell(0, 5, sanitizePDF(stripInlineMD(item)), "", "", false)
			i++
			continue
		}

		// Numbered list (simple)
		if len(trim) > 2 && trim[0] >= '1' && trim[0] <= '9' {
			if dot := strings.Index(trim, ". "); dot > 0 && dot < 4 {
				item := trim[dot+2:]
				pdf.SetFont("Helvetica", "", 10)
				pdf.CellFormat(8, 5, sanitizePDF(trim[:dot+1]), "", 0, "L", false, 0, "")
				pdf.MultiCell(0, 5, sanitizePDF(stripInlineMD(item)), "", "", false)
				i++
				continue
			}
		}

		// Preformatted ASCII diagram block (unfenced): keep line breaks + monospace.
		if looksLikeASCIIArt(line) {
			var art []string
			for i < len(lines) {
				raw := lines[i]
				t := strings.TrimSpace(raw)
				if t == "" {
					break
				}
				if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "```") ||
					strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") ||
					diagramsIsFigure(t) {
					break
				}
				// Stop if this no longer looks like art and previous lines were art.
				if len(art) > 0 && !looksLikeASCIIArt(raw) && !looksLikeASCIIArtContinuation(raw) {
					break
				}
				art = append(art, raw)
				i++
			}
			if len(art) > 0 {
				writeCodeBlock(pdf, "diagram", strings.Join(art, "\n"))
			}
			continue
		}

		// Paragraph: gather consecutive non-empty non-special lines
		var para []string
		for i < len(lines) {
			raw := lines[i]
			t := strings.TrimSpace(raw)
			if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "```") ||
				strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") ||
				diagramsIsFigure(t) || looksLikeASCIIArt(raw) {
				break
			}
			if len(t) > 2 && t[0] >= '1' && t[0] <= '9' {
				if dot := strings.Index(t, ". "); dot > 0 && dot < 4 {
					break
				}
			}
			para = append(para, t)
			i++
		}
		if len(para) > 0 {
			pdf.SetFont("Helvetica", "", 10)
			text := stripInlineMD(strings.Join(para, " "))
			pdf.MultiCell(0, 5.2, sanitizePDF(text), "", "", false)
			pdf.Ln(2)
		}
	}
	return nil
}

func diagramsIsFigure(s string) bool {
	_, ok := diagrams.ParseFigureMarker(s)
	return ok
}

// looksLikeASCIIArt detects monospaced diagram lines (boxes, arrows, alignment).
func looksLikeASCIIArt(line string) bool {
	if line == "" {
		return false
	}
	// Indented non-empty line often starts a preformatted block.
	if (strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")) && strings.TrimSpace(line) != "" {
		// Avoid treating normal blockquotes-ish indent of prose as art unless diagram-ish.
		if hasDiagramChars(line) {
			return true
		}
	}
	return hasDiagramChars(line)
}

func looksLikeASCIIArtContinuation(line string) bool {
	// Keep collecting indented or diagram-character lines.
	if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
		return true
	}
	return hasDiagramChars(line)
}

func hasDiagramChars(line string) bool {
	// Unicode box-drawing
	for _, r := range line {
		if r >= 0x2500 && r <= 0x257F {
			return true
		}
	}
	// Classic ASCII boxes / arrows
	if strings.Contains(line, "+-") || strings.Contains(line, "-+") ||
		strings.Contains(line, "+=") || strings.Contains(line, "->") ||
		strings.Contains(line, "<-") || strings.Contains(line, "|") {
		// Require some structure, not a lone pipe in prose.
		if strings.Count(line, "|") >= 2 || strings.Contains(line, "---") ||
			strings.Contains(line, "===") || strings.Contains(line, "->") ||
			strings.Contains(line, "<-") || strings.Contains(line, "+-") {
			return true
		}
	}
	return false
}

func writeCodeBlock(pdf *fpdf.Fpdf, lang, code string) {
	pdf.Ln(1)
	// Skip decorative lang labels for diagram/ascii/text fences
	showLang := lang != "" && !isDiagramLang(lang)
	if showLang {
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(100, 100, 100)
		pdf.CellFormat(0, 4, sanitizePDF(lang), "", 1, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
	pdf.SetFont("Courier", "", 8)
	for _, line := range strings.Split(code, "\n") {
		// Preserve empty lines for vertical spacing in diagrams.
		pdf.MultiCell(0, 4, sanitizePDF(line), "", "", false)
	}
	pdf.SetFont("Helvetica", "", 10)
	pdf.Ln(2)
}

func isDiagramLang(lang string) bool {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "diagram", "ascii", "text", "art", "":
		return true
	default:
		return false
	}
}

func writeFigure(pdf *fpdf.Fpdf, fig diagrams.Figure) error {
	name := fmt.Sprintf("fig%d", fig.Index)
	opt := fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
	pdf.RegisterImageOptionsReader(name, opt, bytes.NewReader(fig.PNG))
	if pdf.Error() != nil {
		return fmt.Errorf("register figure %d: %w", fig.Index, pdf.Error())
	}

	info := pdf.GetImageInfo(name)
	if info == nil {
		return fmt.Errorf("missing image info for figure %d", fig.Index)
	}

	pageW, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	maxW := pageW - left - right
	imgW := info.Width()
	imgH := info.Height()
	if imgW <= 0 || imgH <= 0 {
		return fmt.Errorf("figure %d has invalid image size %.2fx%.2f", fig.Index, imgW, imgH)
	}

	// Prefer nearly full content width; keep aspect ratio.
	displayW := maxW * 0.95
	scale := displayW / imgW
	displayH := imgH * scale
	// Cap height so one figure doesn't eat the whole page
	if displayH > 120 {
		displayH = 120
		displayW = imgW * (displayH / imgH)
	}

	// Page break if needed
	_, pageH := pdf.GetPageSize()
	_, _, _, bottom := pdf.GetMargins()
	if pdf.GetY()+displayH+10 > pageH-bottom {
		pdf.AddPage()
	}

	x := left + (maxW-displayW)/2
	pdf.ImageOptions(name, x, pdf.GetY(), displayW, displayH, false, opt, 0, "")
	pdf.SetY(pdf.GetY() + displayH + 2)

	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(80, 80, 80)
	pdf.MultiCell(0, 4, sanitizePDF(fig.Caption), "", "C", false)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Helvetica", "", 10)
	pdf.Ln(3)
	return nil
}

// stripInlineMD removes light markdown markers for PDF body text.
func stripInlineMD(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	var b strings.Builder
	inCode := false
	for _, r := range s {
		if r == '`' {
			inCode = !inCode
			continue
		}
		b.WriteRune(r)
	}
	out := b.String()
	for {
		start := strings.Index(out, "[")
		mid := strings.Index(out, "](")
		end := strings.Index(out, ")")
		if start < 0 || mid < 0 || end < 0 || !(start < mid && mid < end) {
			break
		}
		out = out[:start] + out[start+1:mid] + out[end+1:]
	}
	return out
}

func sanitizePDF(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\t':
			b.WriteString("    ")
		case '\r':
			// skip
		case '\n':
			b.WriteByte('\n')
		case 0x00A0: // nbsp
			b.WriteByte(' ')
		case 0x00B7, 0x2022, 0x2023, 0x2043, 0x2219: // · • etc
			b.WriteByte('-')
		case 0x2013, 0x2014, 0x2212: // en/em/minus
			b.WriteByte('-')
		case 0x2018, 0x2019, 0x2032: // curly single quotes / prime
			b.WriteByte('\'')
		case 0x201C, 0x201D: // curly double quotes
			b.WriteByte('"')
		case 0x2026: // ellipsis
			b.WriteString("...")
		case 0x2190: // ←
			b.WriteString("<-")
		case 0x2192, 0x21D2, 0x21A6: // → ⇒ ↦
			b.WriteString("->")
		case 0x2191, 0x25B2: // ↑ ▲
			b.WriteString("^")
		case 0x2193, 0x25BC: // ↓ ▼
			b.WriteString("v")
		case 0x2194: // ↔
			b.WriteString("<->")
		case 0x2264: // ≤
			b.WriteString("<=")
		case 0x2265: // ≥
			b.WriteString(">=")
		case 0x2260: // ≠
			b.WriteString("!=")
		case 0x00D7: // ×
			b.WriteByte('x')
		case 0x03BC, 0x00B5: // μ µ
			b.WriteString("u")
		// Box-drawing → ASCII so monospaced diagrams stay readable.
		case 0x2500, 0x2501, 0x254C, 0x254D, 0x2504, 0x2505: // ─ ━ etc
			b.WriteByte('-')
		case 0x2502, 0x2503, 0x2506, 0x2507, 0x2551: // │ ┃ ║
			b.WriteByte('|')
		case 0x250C, 0x250F, 0x2510, 0x2513, 0x2514, 0x2517, 0x2518, 0x251B,
			0x251C, 0x2523, 0x2524, 0x252B, 0x252C, 0x2533, 0x2534, 0x253B,
			0x253C, 0x254B, 0x2550, 0x2552, 0x2553, 0x2554, 0x2555, 0x2556,
			0x2557, 0x2558, 0x2559, 0x255A, 0x255B, 0x255C, 0x255D, 0x255E,
			0x255F, 0x2560, 0x2561, 0x2562, 0x2563, 0x2564, 0x2565, 0x2566,
			0x2567, 0x2568, 0x2569, 0x256A, 0x256B, 0x256C: // corners/joints
			b.WriteByte('+')
		case 0x2574, 0x2576, 0x2578, 0x257A: // light half-lines
			b.WriteByte('-')
		case 0x2575, 0x2577, 0x2579, 0x257B:
			b.WriteByte('|')
		default:
			if r < 32 {
				continue
			}
			// Core fonts: stick to printable ASCII to avoid mojibake boxes.
			if r > 126 {
				if utf8.ValidRune(r) && r != utf8.RuneError {
					b.WriteByte('?')
				}
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
