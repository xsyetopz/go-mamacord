package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Snowflake is a Discord ID.
//
// Discord snowflakes exceed JavaScript's safe integer range, so the admin API
// encodes them as JSON strings to avoid precision loss in the dashboard.
//
// For defensive compatibility, Snowflake also accepts JSON numbers on input.
type Snowflake uint64

func (s Snowflake) String() string {
	return strconv.FormatUint(uint64(s), 10)
}

func (s Snowflake) MarshalJSON() ([]byte, error) {
	// Always encode as a quoted decimal string.
	return []byte(strconv.Quote(strconv.FormatUint(uint64(s), 10))), nil
}

func (s *Snowflake) UnmarshalJSON(data []byte) error {
	if s == nil {
		return fmt.Errorf("snowflake: nil receiver")
	}
	if bytes.Equal(data, []byte("null")) {
		*s = 0
		return nil
	}

	// First try a string.
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		asString = strings.TrimSpace(asString)
		if asString == "" {
			*s = 0
			return nil
		}
		v, err := strconv.ParseUint(asString, 10, 64)
		if err != nil {
			return fmt.Errorf("snowflake: invalid %q", asString)
		}
		*s = Snowflake(v)
		return nil
	}

	// Then accept numbers defensively.
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		raw := strings.TrimSpace(n.String())
		if raw == "" {
			*s = 0
			return nil
		}
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("snowflake: invalid %q", raw)
		}
		*s = Snowflake(v)
		return nil
	}

	return fmt.Errorf("snowflake: invalid json %q", strings.TrimSpace(string(data)))
}
