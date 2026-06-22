// Package xnotify provides a real-time notification hub backed by Redis Pub/Sub.
package xnotify

import "encoding/json"

// Event type constants.
const (
	EventNew   = "new"   // new notification created
	EventRead  = "read"  // notification marked as read (sync across tabs)
	EventCount = "count" // unread count update
)

// Event is a single notification event pushed to connected clients via SSE.
type Event struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}
