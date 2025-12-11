package domain

import "time"

// SyncStats holds statistics about a sync operation.
type SyncStats struct {
	SourceID  string
	Fetched   int
	New       int
	Updated   int
	Skipped   int
	Errors    int
	Published int
	Duration  time.Duration
}
