package helper_util

import (
	"fmt"
	"time"
)

// Helper function to parse time
func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	return t, err
}

func ParseNullableTime(value interface{}) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	switch v := value.(type) {
	case time.Time:
		return &v, nil
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, err
		}
		return &t, nil
	default:
		return nil, fmt.Errorf("unsupported type for time parsing: %T", value)
	}
}
