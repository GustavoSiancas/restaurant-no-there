package domain

import "errors"

var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource already exists")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInvalidInput = errors.New("invalid input")
)
