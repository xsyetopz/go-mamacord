package scheduling_test

import (
	"testing"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/scheduling"
)

func TestParseScheduleAndNext(t *testing.T) {
	t.Parallel()

	schedule, err := scheduling.ParseSchedule(" 0 9 * * * ")
	if err != nil {
		t.Fatalf("ParseSchedule: %v", err)
	}

	if schedule.Spec() != "0 9 * * *" {
		t.Fatalf("unexpected spec: %q", schedule.Spec())
	}

	loc := time.FixedZone("EEST", 3*60*60)
	from := time.Date(2026, time.April, 2, 5, 30, 0, 0, time.UTC)
	got := schedule.Next(from, loc)
	want := time.Date(2026, time.April, 2, 6, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("unexpected next run: got %s want %s", got, want)
	}
}

func TestParseScheduleRejectsBlank(t *testing.T) {
	t.Parallel()

	if _, err := scheduling.ParseSchedule(" "); err == nil {
		t.Fatalf("expected blank schedule error")
	}
}
