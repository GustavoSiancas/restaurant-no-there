package application

import (
	"context"
	"time"

	"backend/internal/modules/auth/domain"
)

type TokenService interface {
	CreateAccessToken(userID, role string) (string, error)
	HashRefreshToken(token string) string
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *domain.RefreshToken) error
	FindValidByHash(ctx context.Context, hash string, now time.Time) (*domain.RefreshToken, error)
	Revoke(ctx context.Context, id string, revokedAt time.Time) error
	RevokeAllByUser(ctx context.Context, userID string, revokedAt time.Time) error
}
