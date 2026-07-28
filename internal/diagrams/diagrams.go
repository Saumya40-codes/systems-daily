package diagrams

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/draw"
)

const maxSVGPixels = 2000

// Figure is a rasterized diagram extracted from the article.
type Figure struct {
	Index   int
	Kind    string // svg
	Source  string
	PNG     []byte
	Caption string
}

// Result is markdown with diagram fences replaced by figure markers,
// plus rendered PNGs for PDF embedding.
type Result struct {
	Markdown string
	Figures  []Figure
	// Failed is how many diagram fences were dropped (unsupported or render error).
	Failed int
}

var fenceRE = regexp.MustCompile("(?is)```(mermaid|svg)\\s*\\r?\\n(.*?)\\r?\\n```")

func Process(ctx context.Context, markdown string) (Result, error) {
	_ = ctx // reserved if local render ever needs cancel
	var figures []Figure
	var failed int
	out := fenceRE.ReplaceAllStringFunc(markdown, func(match string) string {
		sub := fenceRE.FindStringSubmatch(match)
		if len(sub) != 3 {
			return match
		}
		kind := strings.ToLower(sub[1])
		src := strings.TrimSpace(sub[2])
		if src == "" {
			return match
		}

		if kind == "mermaid" {
			// No remote renderer by design; model should use ASCII or SVG instead.
			failed++
			log.Printf("diagram mermaid skipped (no third-party renderer; prefer ASCII or svg): dropped from PDF")
			return "\n\n"
		}

		idx := len(figures)
		fig := Figure{
			Index:   idx,
			Kind:    kind,
			Source:  src,
			Caption: fmt.Sprintf("Figure %d", idx+1),
		}
		pngBytes, err := renderSVG(src, 900, 600)
		if err != nil {
			failed++
			log.Printf("diagram svg render failed (dropped from PDF): %v", err)
			return "\n\n"
		}
		fig.PNG = pngBytes
		figures = append(figures, fig)
		return fmt.Sprintf("\n\n{{figure:%d}}\n\n", idx)
	})
	return Result{Markdown: out, Figures: figures, Failed: failed}, nil
}

// renderSVG rasterizes SVG to PNG using pure Go (oksvg).
// maxW/maxH are preferred content bounds in pixels; absolute max is maxSVGPixels.
func renderSVG(src string, maxW, maxH int) ([]byte, error) {
	src = stripHardSVG(src)
	icon, err := oksvg.ReadIconStream(strings.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("parse svg: %w", err)
	}
	vb := icon.ViewBox
	w, h := vb.W, vb.H
	if w <= 0 {
		w = 800
	}
	if h <= 0 {
		h = 600
	}
	// Reject absurd viewBoxes before scaling (defense in depth).
	const maxView = 1e6
	if w > maxView || h > maxView {
		return nil, fmt.Errorf("svg viewBox too large: %.0fx%.0f", w, h)
	}
	if maxW <= 0 {
		maxW = 900
	}
	if maxH <= 0 {
		maxH = 600
	}
	if maxW > maxSVGPixels {
		maxW = maxSVGPixels
	}
	if maxH > maxSVGPixels {
		maxH = maxSVGPixels
	}

	scale := float64(maxW) / w
	if s := float64(maxH) / h; s < scale {
		scale = s
	}
	if scale > 2 {
		scale = 2
	}
	outW := int(w * scale)
	outH := int(h * scale)
	if outW < 1 {
		outW = 1
	}
	if outH < 1 {
		outH = 1
	}
	if outW > maxSVGPixels {
		outW = maxSVGPixels
	}
	if outH > maxSVGPixels {
		outH = maxSVGPixels
	}

	icon.SetTarget(0, 0, float64(outW), float64(outH))
	rgba := image.NewRGBA(image.Rect(0, 0, outW, outH))
	draw.Draw(rgba, rgba.Bounds(), image.White, image.Point{}, draw.Src)
	scanner := rasterx.NewScannerGV(outW, outH, rgba, rgba.Bounds())
	dasher := rasterx.NewDasher(outW, outH, scanner)
	icon.Draw(dasher, 1)
	return encodePNG(rgba)
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var styleBlockRE = regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`)

func stripHardSVG(src string) string {
	src = styleBlockRE.ReplaceAllString(src, "")
	lines := strings.Split(src, "\n")
	var keep []string
	for _, ln := range lines {
		if strings.Contains(ln, "@import") {
			continue
		}
		keep = append(keep, ln)
	}
	return strings.Join(keep, "\n")
}

// ParseFigureMarker returns the figure index if s is exactly "{{figure:N}}".
func ParseFigureMarker(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{{figure:") || !strings.HasSuffix(s, "}}") {
		return 0, false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(s, "{{figure:"), "}}")
	n, err := strconv.Atoi(inner)
	if err != nil {
		return 0, false
	}
	return n, true
}
