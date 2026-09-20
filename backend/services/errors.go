package services

import "errors"

// Sentinel errors shared across services.
var (
	ErrNotFound           = errors.New("not found")
	ErrDuplicateUser      = errors.New("email or username already in use")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrForbidden          = errors.New("forbidden")
	ErrVersionConflict    = errors.New("version conflict")
	ErrValidation         = errors.New("validation error")
)
