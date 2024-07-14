package helper_util

import "time"

// Helper function to parse time
func ParseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}