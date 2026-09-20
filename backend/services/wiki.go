package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"

	"github.com/wikicollab/backend/models"
)

// WikiService handles wiki creation and retrieval.
type WikiService struct {
	db *sql.DB
}

// NewWikiService creates a WikiService.
func NewWikiService(db *sql.DB) *WikiService {
	return &WikiService{db: db}
}

// CreateWikiInput holds the fields required to create a wiki.
type CreateWikiInput struct {
	Name    string
	OwnerID string
}

// CreateWiki validates input and inserts a new wiki owned by the given user.
func (s *WikiService) CreateWiki(ctx context.Context, input CreateWikiInput) (*models.Wiki, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: wiki name is required", ErrValidation)
	}
	if len(name) > 255 {
		return nil, fmt.Errorf("%w: wiki name must be 255 characters or fewer", ErrValidation)
	}

	var wiki models.Wiki
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO wikis (name, owner_id)
		 VALUES ($1, $2)
		 RETURNING id, name, owner_id, created_at`,
		name, input.OwnerID,
	).Scan(&wiki.ID, &wiki.Name, &wiki.OwnerID, &wiki.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert wiki: %w", err)
	}

	return &wiki, nil
}

// ListWikis returns all wikis the given user owns or is a member of.
func (s *WikiService) ListWikis(ctx context.Context, ownerID string) ([]models.Wiki, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT w.id, w.name, w.owner_id, w.created_at
		 FROM wikis w
		 LEFT JOIN wiki_members wm ON wm.wiki_id = w.id AND wm.user_id = $1
		 WHERE w.owner_id = $1 OR wm.user_id = $1
		 ORDER BY w.created_at DESC`,
		ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("list wikis: %w", err)
	}
	defer rows.Close()

	var wikis []models.Wiki
	for rows.Next() {
		var w models.Wiki
		if err := rows.Scan(&w.ID, &w.Name, &w.OwnerID, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan wiki: %w", err)
		}
		wikis = append(wikis, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wikis: %w", err)
	}

	return wikis, nil
}

// GetWiki retrieves a wiki by ID.
// Returns ErrNotFound if the wiki does not exist.
func (s *WikiService) GetWiki(ctx context.Context, wikiID string) (*models.Wiki, error) {
	var wiki models.Wiki
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, owner_id, created_at FROM wikis WHERE id = $1`,
		wikiID,
	).Scan(&wiki.ID, &wiki.Name, &wiki.OwnerID, &wiki.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get wiki: %w", err)
	}
	return &wiki, nil
}

// AssertOwner returns ErrForbidden if userID does not own the wiki.
func (s *WikiService) AssertOwner(ctx context.Context, wikiID, userID string) error {
	wiki, err := s.GetWiki(ctx, wikiID)
	if err != nil {
		return err
	}
	if wiki.OwnerID != userID {
		return ErrForbidden
	}
	return nil
}

// AssertAccess returns nil if the user is the owner or a member, ErrForbidden otherwise.
func (s *WikiService) AssertAccess(ctx context.Context, wikiID, userID string) error {
	wiki, err := s.GetWiki(ctx, wikiID)
	if err != nil {
		return err
	}
	if wiki.OwnerID == userID {
		return nil
	}

	var count int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM wiki_members WHERE wiki_id = $1 AND user_id = $2`,
		wikiID, userID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("check wiki membership: %w", err)
	}
	if count == 0 {
		return ErrForbidden
	}
	return nil
}

// shareCodeAlphabet is the set of unambiguous characters used for share codes.
const shareCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateShareCode creates (or returns existing) a 6-char invite code for the wiki.
// Only the wiki owner may call this.
func (s *WikiService) GenerateShareCode(ctx context.Context, wikiID, ownerID string) (string, error) {
	if err := s.AssertOwner(ctx, wikiID, ownerID); err != nil {
		return "", err
	}

	// Return existing code if already generated (idempotent).
	var existing string
	err := s.db.QueryRowContext(ctx,
		`SELECT code FROM share_codes WHERE wiki_id = $1`,
		wikiID,
	).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("check existing share code: %w", err)
	}

	// Generate a random 6-char code.
	code, err := randomCode(6)
	if err != nil {
		return "", fmt.Errorf("generate share code: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO share_codes (wiki_id, code) VALUES ($1, $2)
		 ON CONFLICT (wiki_id) DO NOTHING`,
		wikiID, code,
	)
	if err != nil {
		return "", fmt.Errorf("insert share code: %w", err)
	}

	// Re-read in case another request beat us to it.
	if err := s.db.QueryRowContext(ctx,
		`SELECT code FROM share_codes WHERE wiki_id = $1`, wikiID,
	).Scan(&code); err != nil {
		return "", fmt.Errorf("read share code: %w", err)
	}

	return code, nil
}

// JoinByCode finds a wiki by invite code and upserts the user into wiki_members.
// Returns the wiki on success.
func (s *WikiService) JoinByCode(ctx context.Context, code, userID string) (*models.Wiki, error) {
	var wikiID string
	err := s.db.QueryRowContext(ctx,
		`SELECT wiki_id FROM share_codes WHERE code = $1`,
		strings.ToUpper(strings.TrimSpace(code)),
	).Scan(&wikiID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: invalid share code", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("look up share code: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO wiki_members (wiki_id, user_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		wikiID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("join wiki: %w", err)
	}

	return s.GetWiki(ctx, wikiID)
}

// randomCode returns a random n-character string drawn from shareCodeAlphabet.
func randomCode(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	alphabetLen := byte(len(shareCodeAlphabet))
	result := make([]byte, n)
	for i, v := range b {
		result[i] = shareCodeAlphabet[v%alphabetLen]
	}
	return string(result), nil
}
