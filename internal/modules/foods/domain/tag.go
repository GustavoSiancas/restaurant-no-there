package domain

import coredomain "backend/internal/core/domain"

// Tag is a reusable label that can be assigned to multiple foods.
type Tag struct {
	coredomain.Entity
	Name string `json:"name"`
}
