package postgres

import (
	"context"
	"errors"
	"time"

	core "backend/internal/core/domain"
	"backend/internal/modules/workforce/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

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

func (r *Repository) CreateWorkerInformation(ctx context.Context, i *domain.WorkerInformation) error {
	err := r.db.QueryRow(ctx, `INSERT INTO worker_information (user_id,employee_code,job_title,department,phone,address,hire_date,emergency_contact_name,emergency_contact_phone,notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING created_at,updated_at`, i.UserID, i.EmployeeCode, i.JobTitle, i.Department, i.Phone, i.Address, i.HireDate, i.EmergencyContactName, i.EmergencyContactPhone, i.Notes).Scan(&i.CreatedAt, &i.UpdatedAt)
	return translate(err)
}
func (r *Repository) FindWorkerInformation(ctx context.Context, id string) (*domain.WorkerInformation, error) {
	var i domain.WorkerInformation
	err := r.db.QueryRow(ctx, `SELECT user_id,employee_code,job_title,department,phone,address,hire_date,emergency_contact_name,emergency_contact_phone,notes,created_at,updated_at FROM worker_information WHERE user_id=$1`, id).Scan(&i.UserID, &i.EmployeeCode, &i.JobTitle, &i.Department, &i.Phone, &i.Address, &i.HireDate, &i.EmergencyContactName, &i.EmergencyContactPhone, &i.Notes, &i.CreatedAt, &i.UpdatedAt)
	return &i, translate(err)
}
func (r *Repository) CreateShift(ctx context.Context, s *domain.Shift) error {
	err := r.db.QueryRow(ctx, `INSERT INTO shifts (name,type,description,start_time,end_time) VALUES ($1,$2,$3,$4::time,$5::time) RETURNING id,start_time::text,end_time::text,active,created_at,updated_at`, s.Name, s.Type, s.Description, s.StartTime, s.EndTime).Scan(&s.ID, &s.StartTime, &s.EndTime, &s.Active, &s.CreatedAt, &s.UpdatedAt)
	return translate(err)
}
func scanShift(row pgx.Row) (*domain.Shift, error) {
	var s domain.Shift
	err := row.Scan(&s.ID, &s.Name, &s.Type, &s.Description, &s.StartTime, &s.EndTime, &s.Active, &s.CreatedAt, &s.UpdatedAt)
	return &s, translate(err)
}
func (r *Repository) FindShift(ctx context.Context, id string) (*domain.Shift, error) {
	return scanShift(r.db.QueryRow(ctx, `SELECT id,name,type,description,start_time::text,end_time::text,active,created_at,updated_at FROM shifts WHERE id=$1`, id))
}
func (r *Repository) ListShifts(ctx context.Context) ([]domain.Shift, error) {
	rows, err := r.db.Query(ctx, `SELECT id,name,type,description,start_time::text,end_time::text,active,created_at,updated_at FROM shifts ORDER BY start_time`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.Shift, 0)
	for rows.Next() {
		s, e := scanShift(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, *s)
	}
	return result, rows.Err()
}
func (r *Repository) CreateAssignment(ctx context.Context, a *domain.WorkerShiftAssignment) error {
	err := r.db.QueryRow(ctx, `INSERT INTO worker_shift_assignments (worker_id,shift_id,work_date,assigned_by,notes) VALUES ($1,$2,$3,$4,$5) RETURNING id,created_at,updated_at`, a.WorkerID, a.ShiftID, a.WorkDate, a.AssignedBy, a.Notes).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
	return translate(err)
}
func scanAssignment(row pgx.Row) (*domain.WorkerShiftAssignment, error) {
	var a domain.WorkerShiftAssignment
	err := row.Scan(&a.ID, &a.WorkerID, &a.ShiftID, &a.WorkDate, &a.AssignedBy, &a.Notes, &a.CreatedAt, &a.UpdatedAt)
	return &a, translate(err)
}
func (r *Repository) FindAssignmentByWorkerAndDate(ctx context.Context, workerID string, date time.Time) (*domain.WorkerShiftAssignment, error) {
	return scanAssignment(r.db.QueryRow(ctx, `SELECT id,worker_id,shift_id,work_date,assigned_by,notes,created_at,updated_at FROM worker_shift_assignments WHERE worker_id=$1 AND work_date=$2`, workerID, date))
}
func (r *Repository) ListAssignments(ctx context.Context, from, to time.Time) ([]domain.WorkerShiftAssignment, error) {
	rows, err := r.db.Query(ctx, `SELECT id,worker_id,shift_id,work_date,assigned_by,notes,created_at,updated_at FROM worker_shift_assignments WHERE work_date BETWEEN $1 AND $2 ORDER BY work_date,worker_id`, from, to)
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
