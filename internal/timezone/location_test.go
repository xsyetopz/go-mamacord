package timezone_test

import (
	"testing"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/timezone"
)

func TestLoadLocation(t *testing.T) {
	t.Parallel()

	loc, name, err := timezone.LoadLocation("")
	if err != nil {
		t.Fatalf("LoadLocation(default): %v", err)
	}
	if loc != time.UTC || name != "UTC" {
		t.Fatalf("unexpected default timezone: %v %q", loc, name)
	}

	loc, name, err = timezone.LoadLocation("Europe/Tallinn")
	if err != nil {
		t.Fatalf("LoadLocation(valid): %v", err)
	}
	if loc == nil || name != "Europe/Tallinn" {
		t.Fatalf("unexpected timezone: %v %q", loc, name)
	}

	if _, _, err := timezone.LoadLocation("Not/AZone"); err == nil {
		t.Fatalf("expected invalid timezone error")
	}
}
