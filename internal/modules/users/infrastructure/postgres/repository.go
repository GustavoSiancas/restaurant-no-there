package postgres

import (
	core "backend/internal/core/domain"
	"backend/internal/modules/users/domain"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

const userColumns = `u.id,u.role,u.active,u.last_login_at,u.created_at,u.updated_at`

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Role, &u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	return &u, err
}
func (r *Repository) CreateManagement(ctx context.Context, u *domain.User, p *domain.Profile, username, passwordHash string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	err = tx.QueryRow(ctx, `INSERT INTO users(role) VALUES($1) RETURNING id,active,created_at,updated_at`, u.Role).Scan(&u.ID, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return err
	}
	p.UserID = u.ID
	if _, err = tx.Exec(ctx, `INSERT INTO user_profiles(user_id,first_name,last_name,email) VALUES($1,$2,$3,$4)`, p.UserID, p.FirstName, p.LastName, p.Email); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO user_credentials(user_id,type,identifier,secret_hash) VALUES($1,'PASSWORD',$2,$3)`, u.ID, username, passwordHash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *Repository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users u WHERE u.id=$1`, id))
}
func (r *Repository) FindPasswordCredential(ctx context.Context, value string) (*domain.User, string, error) {
	var u domain.User
	var hash string
	err := r.db.QueryRow(ctx, `SELECT `+userColumns+`,c.secret_hash FROM users u JOIN user_credentials c ON c.user_id=u.id AND c.type='PASSWORD' AND c.active=TRUE WHERE LOWER(c.identifier)=LOWER($1)`, value).Scan(&u.ID, &u.Role, &u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", core.ErrNotFound
	}
	return &u, hash, err
}
func (r *Repository) FindByDNI(ctx context.Context, value string) (*domain.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users u JOIN user_credentials c ON c.user_id=u.id AND c.type='DNI' AND c.active=TRUE WHERE c.identifier=$1`, value))
}
func (r *Repository) List(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, `SELECT `+userColumns+` FROM users u ORDER BY u.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.User, 0)
	for rows.Next() {
		u, e := scanUser(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, *u)
	}
	return items, rows.Err()
}
func (r *Repository) RoleExists(ctx context.Context, role domain.Role) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE role=$1)`, role).Scan(&exists)
	return exists, err
}
