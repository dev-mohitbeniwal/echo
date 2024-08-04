package model

import "time"

type CacheKey struct {
	SubjectID  string
	ResourceID string
	Action     string
}

type CacheEntry struct {
	Decision  AccessDecision
	ExpiresAt time.Time
}
