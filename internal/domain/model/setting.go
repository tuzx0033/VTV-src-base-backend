package model

import "time"

// Setting is a system-wide key-value configuration entry.
type Setting struct {
	Key         string
	Value       string
	Description string
	UpdatedAt   time.Time
	UpdatedBy   *int64
}
