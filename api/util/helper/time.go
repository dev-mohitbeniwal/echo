package helper_util

import "time"

// Helper function to parse time
func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	return t, err
}
