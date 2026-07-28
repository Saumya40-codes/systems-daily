package site

import (
	"regexp"
	"strings"
)

var svgBlockRE = regexp.MustCompile(`(?is)<svg\b[^>]*>.*?</svg>`)

// filterUselessSVGs drops decorative placeholder SVGs models often emit
// (single gray rect + a couple of text labels, no structure).
func filterUselessSVGs(htmlFrag string) string {
	return svgBlockRE.ReplaceAllStringFunc(htmlFrag, func(svg string) string {
		if isUsefulSVG(svg) {
			return svg
		}
		return ""
	})
}

func isUsefulSVG(svg string) bool {
	low := strings.ToLower(svg)
	// Must look like an actual diagram: multiple shapes or lines/paths.
	shapes := 0
	for _, tag := range []string{"<rect", "<line", "<path", "<circle", "<ellipse", "<polygon", "<polyline"} {
		shapes += strings.Count(low, tag)
	}
	texts := strings.Count(low, "<text")
	// Placeholder pattern: 0-1 shape, mostly labels
	if shapes <= 1 && texts <= 3 {
		return false
	}
	// Single fat rect filling most of the canvas is almost always a stub
	if shapes == 1 && strings.Count(low, "<rect") == 1 && texts <= 2 {
		return false
	}
	// Need some structural connectivity or multiple elements
	if shapes+texts < 4 {
		return false
	}
	return true
}
