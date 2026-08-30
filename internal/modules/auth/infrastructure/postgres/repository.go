package postgres

import (
	core "backend/internal/core/domain"
	"backend/internal/modules/auth/domain"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Repository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }
func (r *Repository) Create(ctx context.Context, t *domain.RefreshToken) error {
	return r.db.QueryRow(ctx, `INSERT INTO refresh_tokens (user_id,token_hash,expires_at,user_agent,ip_address) VALUES ($1,$2,$3,$4,NULLIF($5,'')::inet) RETURNING id,created_at,updated_at`, t.UserID, t.TokenHash, t.ExpiresAt, t.UserAgent, t.IPAddress).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}
func (r *Repository) FindValidByHash(ctx context.Context, hash string, now time.Time) (*domain.RefreshToken, error) {
	var t domain.RefreshToken
	err := r.db.QueryRow(ctx, `SELECT id,user_id,token_hash,expires_at,revoked_at,user_agent,COALESCE(host(ip_address),''),created_at,updated_at FROM refresh_tokens WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>$2`, hash, now).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.UserAgent, &t.IPAddress, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, core.ErrUnauthorized
	}
	return &t, err
}
func (r *Repository) Revoke(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=$2,updated_at=NOW() WHERE id=$1`, id, at)
	return err
}
func (r *Repository) RevokeAllByUser(ctx context.Context, userID string, at time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=$2,updated_at=NOW() WHERE user_id=$1 AND revoked_at IS NULL`, userID, at)
	return err
}
