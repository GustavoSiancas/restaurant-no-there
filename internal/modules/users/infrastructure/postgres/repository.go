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
func (r *Repository) FindMyUser(ctx context.Context, id string) (*domain.MyUser, error) {
	var result domain.MyUser
	err := r.db.QueryRow(ctx, `SELECT `+userColumns+`,p.user_id,p.first_name,p.last_name,p.email FROM users u JOIN user_profiles p ON p.user_id=u.id WHERE u.id=$1`, id).Scan(&result.ID, &result.Role, &result.Active, &result.LastLoginAt, &result.CreatedAt, &result.UpdatedAt, &result.Profile.UserID, &result.Profile.FirstName, &result.Profile.LastName, &result.Profile.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `SELECT type,CASE WHEN type='DNI' THEN REPEAT('*',GREATEST(LENGTH(identifier)-4,0)) || RIGHT(identifier,4) ELSE identifier END FROM user_credentials WHERE user_id=$1 AND active=TRUE ORDER BY type`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result.Credentials = make([]domain.PublicCredential, 0)
	for rows.Next() {
		var credential domain.PublicCredential
		if err = rows.Scan(&credential.Type, &credential.Identifier); err != nil {
			return nil, err
		}
		result.Credentials = append(result.Credentials, credential)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if result.Role == domain.RoleWorker {
		var worker domain.WorkerDetails
		err = r.db.QueryRow(ctx, `SELECT employee_code,job_title,department,phone,address,hire_date,emergency_contact_name,emergency_contact_phone,notes FROM worker_information WHERE user_id=$1`, id).Scan(&worker.EmployeeCode, &worker.JobTitle, &worker.Department, &worker.Phone, &worker.Address, &worker.HireDate, &worker.EmergencyContactName, &worker.EmergencyContactPhone, &worker.Notes)
		if err == nil {
			result.Worker = &worker
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	return &result, nil
}
func (r *Repository) ListByRoles(ctx context.Context, roles ...domain.Role) ([]domain.MyUser, error) {
	roleNames := make([]string, len(roles))
	for index, role := range roles {
		roleNames[index] = string(role)
	}
	rows, err := r.db.Query(ctx, `SELECT id FROM users WHERE role=ANY($1::user_role[]) ORDER BY created_at DESC`, roleNames)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	result := make([]domain.MyUser, 0, len(ids))
	for _, id := range ids {
		user, findErr := r.FindMyUser(ctx, id)
		if findErr != nil {
			return nil, findErr
		}
		result = append(result, *user)
	}
	return result, nil
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
func (r *Repository) RoleExists(ctx context.Context, role domain.Role) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE role=$1)`, role).Scan(&exists)
	return exists, err
}
