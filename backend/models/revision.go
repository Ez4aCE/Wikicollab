package models

import "time"

// Revision records a snapshot of page content at a given version.
type Revision struct {
	ID        string    `json:"id"`
	PageID    string    `json:"page_id"`
	Content   string    `json:"content"`
	Version   int       `json:"version"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}
