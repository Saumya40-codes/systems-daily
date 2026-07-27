package schedule

import (
	"testing"
	"time"
)

func TestNextAt(t *testing.T) {
	loc := time.FixedZone("test", 0)
	now := time.Date(2026, 3, 15, 8, 0, 0, 0, loc)
	next := nextAt(now, 9, 0)
	want := time.Date(2026, 3, 15, 9, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("got %v want %v", next, want)
	}

	now = time.Date(2026, 3, 15, 9, 0, 0, 0, loc)
	next = nextAt(now, 9, 0)
	want = time.Date(2026, 3, 16, 9, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("after exact time: got %v want %v", next, want)
	}
}
