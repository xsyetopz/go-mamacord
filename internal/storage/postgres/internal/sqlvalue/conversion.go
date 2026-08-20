package sqlvalue

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
)

func Uint64ToInt64(v uint64, field string) (int64, error) {
	if v > uint64(math.MaxInt64) {
		return 0, fmt.Errorf("%s overflows int64", field)
	}
	return int64(v), nil
}

func Int64ToUint64(v int64, field string) (uint64, error) {
	if v < 0 {
		return 0, fmt.Errorf("%s is negative", field)
	}
	return uint64(v), nil
}

func OptionalUint64(v *uint64, field string) (any, error) {
	if v == nil {
		return sql.NullInt64{}, nil
	}
	n, err := Uint64ToInt64(*v, field)
	if err != nil {
		return nil, err
	}
	return sql.NullInt64{Int64: n, Valid: true}, nil
}

func NullIfBlank(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}
