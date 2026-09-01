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
	err := r.db.QueryRow(ctx, `INSERT INTO worker_shift_assignments (worker_id,shift_type,work_date,assigned_by,notes) VALUES ($1,$2,$3,$4,$5) RETURNING id,status,created_at,updated_at`, a.WorkerID, a.ShiftType, a.WorkDate, a.AssignedBy, a.Notes).Scan(&a.ID, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	return translate(err)
}

func (r *Repository) ReplaceOpenAssignments(ctx context.Context, workerIDs []string, shiftType domain.ShiftType, from, to time.Time, assignedBy string) (created, replaced int, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for date := from; !date.After(to); date = date.AddDate(0, 0, 1) {
		for _, workerID := range workerIDs {
			result, execErr := tx.Exec(ctx, `DELETE FROM worker_shift_assignments WHERE worker_id=$1 AND work_date=$2 AND status='OPEN'`, workerID, date)
			if execErr != nil {
				return 0, 0, translate(execErr)
			}
			replaced += int(result.RowsAffected())
			if _, execErr = tx.Exec(ctx, `INSERT INTO worker_shift_assignments (worker_id,shift_type,work_date,assigned_by) VALUES ($1,$2,$3,$4)`, workerID, shiftType, date, assignedBy); execErr != nil {
				return 0, 0, translate(execErr)
			}
			created++
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return created, replaced, nil
}
func (r *Repository) FindAssignmentByID(ctx context.Context, id string) (*domain.WorkerShiftAssignment, error) {
	return scanAssignment(r.db.QueryRow(ctx, `SELECT id,worker_id,shift_type,status,work_date,assigned_by,notes,created_at,updated_at FROM worker_shift_assignments WHERE id=$1`, id))
}
func (r *Repository) UpdateAssignment(ctx context.Context, a *domain.WorkerShiftAssignment) error {
	err := r.db.QueryRow(ctx, `UPDATE worker_shift_assignments SET shift_type=$2,work_date=$3,notes=$4,updated_at=NOW() WHERE id=$1 AND status='OPEN' RETURNING updated_at`, a.ID, a.ShiftType, a.WorkDate, a.Notes).Scan(&a.UpdatedAt)
	return translate(err)
}
func (r *Repository) DeleteAssignment(ctx context.Context, id string, today time.Time) error {
	result, err := r.db.Exec(ctx, `DELETE FROM worker_shift_assignments WHERE id=$1 AND status='OPEN'`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return core.ErrLocked
	}
	return nil
}
func scanAssignment(row pgx.Row) (*domain.WorkerShiftAssignment, error) {
	var a domain.WorkerShiftAssignment
	err := row.Scan(&a.ID, &a.WorkerID, &a.ShiftType, &a.Status, &a.WorkDate, &a.AssignedBy, &a.Notes, &a.CreatedAt, &a.UpdatedAt)
	return &a, translate(err)
}
func (r *Repository) FindAssignmentByWorkerAndDate(ctx context.Context, workerID string, date time.Time) (*domain.WorkerShiftAssignment, error) {
	return scanAssignment(r.db.QueryRow(ctx, `SELECT id,worker_id,shift_type,status,work_date,assigned_by,notes,created_at,updated_at FROM worker_shift_assignments WHERE worker_id=$1 AND work_date=$2`, workerID, date))
}
func (r *Repository) ListWorkerAssignmentsRange(ctx context.Context, workerID string, from, to time.Time) ([]domain.WorkerShiftAssignment, error) {
	rows, err := r.db.Query(ctx, `SELECT id,worker_id,shift_type,status,work_date,assigned_by,notes,created_at,updated_at FROM worker_shift_assignments WHERE worker_id=$1 AND work_date BETWEEN $2 AND $3 ORDER BY work_date ASC`, workerID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.WorkerShiftAssignment, 0)
	for rows.Next() {
		item, e := scanAssignment(rows)
		if e != nil {
			return nil, e
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *Repository) ListShiftPreview(
	ctx context.Context,
	date time.Time,
) ([]domain.ShiftPreviewRow, error) {

	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			a.id,
			a.shift_type,
			a.work_date,
			a.worker_id,
			BTRIM(p.first_name || ' ' || p.last_name),
			REPEAT('*', GREATEST(LENGTH(dni.identifier) - 4, 0))
				|| RIGHT(dni.identifier, 4),
			wi.employee_code,
			wi.job_title,
			wi.department
		FROM worker_shift_assignments a

		JOIN user_profiles p
			ON p.user_id = a.worker_id

		JOIN worker_information wi
			ON wi.user_id = a.worker_id

		JOIN user_credentials dni
			ON dni.user_id = a.worker_id
			AND dni.type = 'DNI'
			AND dni.active = TRUE

		WHERE a.work_date = $1
			OR (a.shift_type = 'NIGHT' AND a.work_date = ($1::date - 1))

		ORDER BY
			a.work_date,
			p.first_name,
			p.last_name
		`,
		date,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	items := make([]domain.ShiftPreviewRow, 0)

	for rows.Next() {
		var item domain.ShiftPreviewRow

		if err = rows.Scan(
			&item.AssignmentID,
			&item.ShiftType,
			&item.WorkDate,
			&item.Worker.ID,
			&item.Worker.FullName,
			&item.Worker.DocumentNumber,
			&item.Worker.EmployeeCode,
			&item.Worker.JobTitle,
			&item.Worker.Department,
		); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *Repository) ListActiveMealRules(ctx context.Context) ([]domain.PreviewRule, error) {
	rows, err := r.db.Query(ctx, `SELECT meal_type::text,claim_start::text,claim_end::text FROM meal_service_rules WHERE active=TRUE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.PreviewRule, 0)
	for rows.Next() {
		var item domain.PreviewRule
		if err = rows.Scan(&item.MealType, &item.Start, &item.End); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
