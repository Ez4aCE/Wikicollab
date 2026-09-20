package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/wikicollab/backend/models"
)

// RevisionService handles revision retrieval.
type RevisionService struct {
	db *sql.DB
}

// NewRevisionService creates a RevisionService.
func NewRevisionService(db *sql.DB) *RevisionService {
	return &RevisionService{db: db}
}

// ListRevisions returns all revisions for a page, ordered newest first.
func (s *RevisionService) ListRevisions(ctx context.Context, pageID string) ([]models.Revision, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, page_id, content, version, created_by, created_at
		 FROM revisions
		 WHERE page_id = $1
		 ORDER BY version DESC`,
		pageID,
	)
	if err != nil {
		return nil, fmt.Errorf("list revisions: %w", err)
	}
	defer rows.Close()

	var revisions []models.Revision
	for rows.Next() {
		var rev models.Revision
		if err := rows.Scan(&rev.ID, &rev.PageID, &rev.Content, &rev.Version, &rev.CreatedBy, &rev.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan revision: %w", err)
		}
		revisions = append(revisions, rev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate revisions: %w", err)
	}

	return revisions, nil
}

// GetRevision fetches a single revision by ID.
func (s *RevisionService) GetRevision(ctx context.Context, revisionID string) (*models.Revision, error) {
	var rev models.Revision
	err := s.db.QueryRowContext(ctx,
		`SELECT id, page_id, content, version, created_by, created_at
		 FROM revisions WHERE id = $1`,
		revisionID,
	).Scan(&rev.ID, &rev.PageID, &rev.Content, &rev.Version, &rev.CreatedBy, &rev.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get revision: %w", err)
	}
	return &rev, nil
}
