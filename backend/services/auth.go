package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/wikicollab/backend/models"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// AuthService handles user registration and login logic.
type AuthService struct {
	db *sql.DB
}

// NewAuthService creates an AuthService.
func NewAuthService(db *sql.DB) *AuthService {
	return &AuthService{db: db}
}

// RegisterInput holds the fields required to register a new user.
type RegisterInput struct {
	Username string
	Email    string
	Password string
}

// LoginInput holds the fields required to log in.
type LoginInput struct {
	Email    string
	Password string
}

// Register validates input, hashes the password, and inserts a new user.
// Returns the created user or a descriptive error.
func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*models.User, error) {
	if err := validateRegister(input); err != nil {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	var user models.User
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO users (username, email, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, username, email, created_at`,
		input.Username, strings.ToLower(input.Email), string(hash),
	).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateUser
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return &user, nil
}

// Login verifies credentials and returns the user on success.
func (s *AuthService) Login(ctx context.Context, input LoginInput) (*models.User, error) {
	if strings.TrimSpace(input.Email) == "" || strings.TrimSpace(input.Password) == "" {
		return nil, ErrInvalidCredentials
	}

	var user models.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, email, password_hash, created_at
		 FROM users WHERE email = $1`,
		strings.ToLower(input.Email),
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return &user, nil
}

// GetUserByID fetches a user by their ID.
func (s *AuthService) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, username, email, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &user, nil
}

func validateRegister(input RegisterInput) error {
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(input.Email)
	input.Password = strings.TrimSpace(input.Password)

	if input.Username == "" {
		return errors.New("username is required")
	}
	if len(input.Username) < 2 || len(input.Username) > 50 {
		return errors.New("username must be between 2 and 50 characters")
	}
	if input.Email == "" {
		return errors.New("email is required")
	}
	if !emailRegex.MatchString(input.Email) {
		return errors.New("invalid email format")
	}
	if input.Password == "" {
		return errors.New("password is required")
	}
	if len(input.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unique")
}
