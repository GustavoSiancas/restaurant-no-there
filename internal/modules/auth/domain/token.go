package domain

import (
	"time"

	core "backend/internal/core/domain"
)

type RefreshToken struct {
	core.Entity
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	UserAgent string
	IPAddress string
}

func (t RefreshToken) IsValid(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}
