package postgres

import (
	"context"
	"errors"

	core "backend/internal/core/domain"
	"backend/internal/modules/users/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

const userColumns = `id, username, dni, email, password_hash, first_name, last_name, role, active, last_login_at, created_at, updated_at`

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Username, &u.DNI, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName, &u.Role, &u.Active, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	return &u, err
}
func (r *Repository) Create(ctx context.Context, u *domain.User) error {
	q := `INSERT INTO users (username,dni,email,password_hash,first_name,last_name,role) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING ` + userColumns
	created, err := scanUser(r.db.QueryRow(ctx, q, u.Username, u.DNI, u.Email, u.PasswordHash, u.FirstName, u.LastName, u.Role))
	if err == nil {
		*u = *created
	}
	return err
}
func (r *Repository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id=$1`, id))
}
func (r *Repository) FindByUsername(ctx context.Context, v string) (*domain.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE LOWER(username)=LOWER($1)`, v))
}
func (r *Repository) FindByDNI(ctx context.Context, v string) (*domain.User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE dni=$1`, v))
}
func (r *Repository) List(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]domain.User, 0)
	for rows.Next() {
		u, e := scanUser(rows)
		if e != nil {
			return nil, e
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}
func (r *Repository) RoleExists(ctx context.Context, role domain.Role) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE role=$1)`, role).Scan(&exists)
	return exists, err
}
