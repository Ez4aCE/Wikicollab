package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/wikicollab/backend/models"
)

// PageService handles page creation, retrieval, and updates.
// It is intentionally decoupled from HTTP so that Member 2 can call it from WebSocket handlers too.
type PageService struct {
	db *sql.DB
}

// NewPageService creates a PageService.
func NewPageService(db *sql.DB) *PageService {
	return &PageService{db: db}
}

// CreatePageInput holds the fields needed to create a new page.
type CreatePageInput struct {
	WikiID    string
	Title     string
	Content   string
	CreatedBy string
}

// UpdatePageInput holds the fields needed to update an existing page.
type UpdatePageInput struct {
	PageID          string
	Title           string
	Content         string
	ExpectedVersion int
	UpdatedBy       string
}

// CreatePage inserts a new page at version 1 and records the initial revision.
// Both the page insert and revision insert happen inside a single transaction.
func (s *PageService) CreatePage(ctx context.Context, input CreatePageInput) (*models.Page, error) {
	if err := validatePage(input.Title, input.Content); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var page models.Page
	err = tx.QueryRowContext(ctx,
		`INSERT INTO pages (wiki_id, title, content, version)
		 VALUES ($1, $2, $3, 1)
		 RETURNING id, wiki_id, title, content, version, created_at, updated_at`,
		input.WikiID, strings.TrimSpace(input.Title), input.Content,
	).Scan(&page.ID, &page.WikiID, &page.Title, &page.Content, &page.Version, &page.CreatedAt, &page.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert page: %w", err)
	}

	// Record initial revision
	_, err = tx.ExecContext(ctx,
		`INSERT INTO revisions (page_id, content, version, created_by)
		 VALUES ($1, $2, $3, $4)`,
		page.ID, page.Content, page.Version, input.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert initial revision: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &page, nil
}

// GetPage retrieves a single page by ID.
func (s *PageService) GetPage(ctx context.Context, pageID string) (*models.Page, error) {
	var page models.Page
	err := s.db.QueryRowContext(ctx,
		`SELECT id, wiki_id, title, content, version, created_at, updated_at
		 FROM pages WHERE id = $1`,
		pageID,
	).Scan(&page.ID, &page.WikiID, &page.Title, &page.Content, &page.Version, &page.CreatedAt, &page.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get page: %w", err)
	}
	return &page, nil
}

// ListPages returns all pages belonging to a wiki.
func (s *PageService) ListPages(ctx context.Context, wikiID string) ([]models.Page, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, wiki_id, title, content, version, created_at, updated_at
		 FROM pages WHERE wiki_id = $1
		 ORDER BY created_at DESC`,
		wikiID,
	)
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	defer rows.Close()

	var pages []models.Page
	for rows.Next() {
		var p models.Page
		if err := rows.Scan(&p.ID, &p.WikiID, &p.Title, &p.Content, &p.Version, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		pages = append(pages, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pages: %w", err)
	}

	return pages, nil
}

// UpdatePage performs an optimistic-concurrency update of a page.
// It verifies that the current version matches expected_version, then increments the version
// and records a new revision — all within a single transaction.
func (s *PageService) UpdatePage(ctx context.Context, input UpdatePageInput) (*models.Page, error) {
	if err := validatePage(input.Title, input.Content); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Lock the row and check the version atomically.
	var currentVersion int
	err = tx.QueryRowContext(ctx,
		`SELECT version FROM pages WHERE id = $1 FOR UPDATE`,
		input.PageID,
	).Scan(&currentVersion)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock page: %w", err)
	}

	if currentVersion != input.ExpectedVersion {
		return nil, ErrVersionConflict
	}

	newVersion := currentVersion + 1

	var page models.Page
	err = tx.QueryRowContext(ctx,
		`UPDATE pages
		 SET title = $1, content = $2, version = $3, updated_at = NOW()
		 WHERE id = $4
		 RETURNING id, wiki_id, title, content, version, created_at, updated_at`,
		strings.TrimSpace(input.Title), input.Content, newVersion, input.PageID,
	).Scan(&page.ID, &page.WikiID, &page.Title, &page.Content, &page.Version, &page.CreatedAt, &page.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update page: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO revisions (page_id, content, version, created_by)
		 VALUES ($1, $2, $3, $4)`,
		page.ID, page.Content, page.Version, input.UpdatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert revision: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &page, nil
}

func validatePage(title, content string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("%w: title is required", ErrValidation)
	}
	if len(strings.TrimSpace(title)) > 255 {
		return fmt.Errorf("%w: title must be 255 characters or fewer", ErrValidation)
	}
	if content == "" {
		return fmt.Errorf("%w: content is required", ErrValidation)
	}
	return nil
}
