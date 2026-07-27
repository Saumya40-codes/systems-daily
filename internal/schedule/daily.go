package schedule

import (
	"context"
	"log"
	"time"
)

// Daily runs fn once per day at the given wall-clock time (HH:MM) in loc.
// It runs fn immediately if fireNow is true, then waits for the next occurrence.
func Daily(ctx context.Context, at string, loc *time.Location, fireNow bool, fn func(context.Context) error) error {
	if loc == nil {
		loc = time.Local
	}
	hour, minute, err := parseHHMM(at)
	if err != nil {
		return err
	}

	if fireNow {
		log.Printf("running job immediately (--now)")
		if err := fn(ctx); err != nil {
			log.Printf("job error: %v", err)
		}
	}

	for {
		next := nextAt(time.Now().In(loc), hour, minute)
		delay := time.Until(next)
		log.Printf("next send at %s (in %s)", next.Format(time.RFC1123), delay.Round(time.Second))

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			if err := fn(ctx); err != nil {
				log.Printf("job error: %v", err)
				// continue schedule even on failure
			}
		}
	}
}

func parseHHMM(s string) (hour, minute int, err error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, 0, err
	}
	return t.Hour(), t.Minute(), nil
}

func nextAt(now time.Time, hour, minute int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
