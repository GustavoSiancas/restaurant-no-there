package postgres

import (
	"context"
	"errors"
	"time"

	core "backend/internal/core/domain"
	userdomain "backend/internal/modules/users/domain"
	"backend/internal/modules/workforce/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) CreateWorker(ctx context.Context, u *userdomain.User, i *domain.WorkerInformation, dni string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	err = tx.QueryRow(ctx, `INSERT INTO users (role) VALUES ('WORKER') RETURNING id,active,created_at,updated_at`).Scan(&u.ID, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return translate(err)
	}
	i.UserID = u.ID
	if _, err = tx.Exec(ctx, `INSERT INTO user_profiles(user_id,first_name,last_name,email) VALUES($1,$2,$3,$4)`, u.ID, i.FirstName, i.LastName, i.Email); err != nil {
		return translate(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO user_credentials(user_id,type,identifier) VALUES($1,'DNI',$2)`, u.ID, dni); err != nil {
		return translate(err)
	}
	err = tx.QueryRow(ctx, `INSERT INTO worker_information (user_id,employee_code,job_title,department,phone,address,hire_date,emergency_contact_name,emergency_contact_phone,notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING created_at,updated_at`, i.UserID, i.EmployeeCode, i.JobTitle, i.Department, i.Phone, i.Address, i.HireDate, i.EmergencyContactName, i.EmergencyContactPhone, i.Notes).Scan(&i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		return translate(err)
	}
	return tx.Commit(ctx)
}

func translate(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return core.ErrConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return core.ErrNotFound
	}
	return err
}

func (r *Repository) FindWorkerInformation(ctx context.Context, id string) (*domain.WorkerInformation, error) {
	var i domain.WorkerInformation
	err := r.db.QueryRow(ctx, `SELECT wi.user_id,p.first_name,p.last_name,p.email,wi.employee_code,wi.job_title,wi.department,wi.phone,wi.address,wi.hire_date,wi.emergency_contact_name,wi.emergency_contact_phone,wi.notes,wi.created_at,wi.updated_at FROM worker_information wi JOIN user_profiles p ON p.user_id=wi.user_id WHERE wi.user_id=$1`, id).Scan(&i.UserID, &i.FirstName, &i.LastName, &i.Email, &i.EmployeeCode, &i.JobTitle, &i.Department, &i.Phone, &i.Address, &i.HireDate, &i.EmergencyContactName, &i.EmergencyContactPhone, &i.Notes, &i.CreatedAt, &i.UpdatedAt)
	return &i, translate(err)
}
func (r *Repository) CreateAssignment(ctx context.Context, a *domain.WorkerShiftAssignment) error {
	err := r.db.QueryRow(ctx, `INSERT INTO worker_shift_assignments (worker_id,shift_type,work_date,assigned_by,notes) VALUES ($1,$2,$3,$4,$5) RETURNING id,created_at,updated_at`, a.WorkerID, a.ShiftType, a.WorkDate, a.AssignedBy, a.Notes).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
	return translate(err)
}
func (r *Repository) FindAssignmentByID(ctx context.Context, id string) (*domain.WorkerShiftAssignment, error) {
	return scanAssignment(r.db.QueryRow(ctx, `SELECT id,worker_id,shift_type,work_date,assigned_by,notes,created_at,updated_at FROM worker_shift_assignments WHERE id=$1`, id))
}
func (r *Repository) UpdateAssignment(ctx context.Context, a *domain.WorkerShiftAssignment) error {
	err := r.db.QueryRow(ctx, `UPDATE worker_shift_assignments SET shift_type=$2,work_date=$3,notes=$4,updated_at=NOW() WHERE id=$1 RETURNING updated_at`, a.ID, a.ShiftType, a.WorkDate, a.Notes).Scan(&a.UpdatedAt)
	return translate(err)
}
func scanAssignment(row pgx.Row) (*domain.WorkerShiftAssignment, error) {
	var a domain.WorkerShiftAssignment
	err := row.Scan(&a.ID, &a.WorkerID, &a.ShiftType, &a.WorkDate, &a.AssignedBy, &a.Notes, &a.CreatedAt, &a.UpdatedAt)
	return &a, translate(err)
}
func (r *Repository) FindAssignmentByWorkerAndDate(ctx context.Context, workerID string, date time.Time) (*domain.WorkerShiftAssignment, error) {
	return scanAssignment(r.db.QueryRow(ctx, `SELECT id,worker_id,shift_type,work_date,assigned_by,notes,created_at,updated_at FROM worker_shift_assignments WHERE worker_id=$1 AND work_date=$2`, workerID, date))
}
func (r *Repository) ListAssignments(ctx context.Context, from, to time.Time) ([]domain.WorkerShiftAssignment, error) {
	rows, err := r.db.Query(ctx, `SELECT id,worker_id,shift_type,work_date,assigned_by,notes,created_at,updated_at FROM worker_shift_assignments WHERE work_date BETWEEN $1 AND $2 ORDER BY work_date,worker_id`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.WorkerShiftAssignment, 0)
	for rows.Next() {
		a, e := scanAssignment(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, *a)
	}
	return result, rows.Err()
}
