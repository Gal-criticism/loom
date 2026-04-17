package store

import "errors"

// Common errors
var (
	ErrSessionNotFound = errors.New("session not found")
	ErrInvalidSession  = errors.New("invalid session")
	ErrInvalidPath     = errors.New("invalid path")
	ErrInvalidRuntime  = errors.New("invalid runtime")
	ErrSessionExists   = errors.New("session already exists")
	ErrStoreClosed     = errors.New("store is closed")
)
