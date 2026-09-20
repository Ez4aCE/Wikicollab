package models

import "time"

// Page represents a Markdown document within a wiki.
type Page struct {
	ID        string    `json:"id"`
	WikiID    string    `json:"wiki_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
