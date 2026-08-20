package timezone

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const DefaultTimezone = "UTC"

func LoadLocation(tz string) (*time.Location, string, error) {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		tz = DefaultTimezone
	}
	if strings.EqualFold(tz, "utc") {
		return time.UTC, "UTC", nil
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, "", fmt.Errorf("invalid timezone %q: %w", tz, err)
	}
	name := strings.TrimSpace(loc.String())
	if name == "" {
		return nil, "", errors.New("invalid timezone")
	}
	return loc, name, nil
}
