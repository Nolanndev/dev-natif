package store

import "github.com/Nolanndev/dev-natif/internal/domain"

// Package-level aliases so all repository files reference the same sentinel
// values without repeating the import path.
var (
	errNotFound = domain.ErrNotFound
	errConflict = domain.ErrConflict
)
