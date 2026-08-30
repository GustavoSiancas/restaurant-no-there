package postgres

import (
	"context"
	"errors"
	"time"

	core "backend/internal/core/domain"
	"backend/internal/modules/meals/domain"
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

func (r *Repository) FindRule(ctx context.Context, t domain.MealType) (*domain.ServiceRule, error) {
	var rule domain.ServiceRule
	err := r.db.QueryRow(ctx, `SELECT meal_type,claim_start::text,claim_end::text,timezone,description,active FROM meal_service_rules WHERE meal_type=$1`, t).Scan(&rule.MealType, &rule.ClaimStart, &rule.ClaimEnd, &rule.Timezone, &rule.Description, &rule.Active)
	return &rule, translate(err)
}
func (r *Repository) ListRules(ctx context.Context) ([]domain.ServiceRule, error) {
	rows, err := r.db.Query(ctx, `SELECT meal_type,claim_start::text,claim_end::text,timezone,description,active FROM meal_service_rules WHERE active=TRUE ORDER BY claim_start`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.ServiceRule, 0)
	for rows.Next() {
		var rule domain.ServiceRule
		if err = rows.Scan(&rule.MealType, &rule.ClaimStart, &rule.ClaimEnd, &rule.Timezone, &rule.Description, &rule.Active); err != nil {
			return nil, err
		}
		items = append(items, rule)
	}
	return items, rows.Err()
}
func (r *Repository) FindEligibleAssignment(ctx context.Context, workerID string, mealType domain.MealType, date time.Time) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `SELECT a.id FROM worker_shift_assignments a
		WHERE a.worker_id=$1 AND (
			($2='DESAYUNO' AND ((a.shift_type='DIA' AND a.work_date=$3) OR (a.shift_type='NOCHE' AND a.work_date=($3::date - 1))))
			OR ($2='TARDE' AND a.shift_type='DIA' AND a.work_date=$3)
			OR ($2='NOCHE' AND a.shift_type='NOCHE' AND a.work_date=$3)
		) LIMIT 1`, workerID, mealType, date).Scan(&id)
	return id, translate(err)
}
func (r *Repository) FindCurrentShift(ctx context.Context, workerID, shiftType string, date time.Time) (*domain.CurrentShift, error) {
	var shift domain.CurrentShift
	err := r.db.QueryRow(ctx, `SELECT id,shift_type,work_date FROM worker_shift_assignments WHERE worker_id=$1 AND shift_type=$2 AND work_date=$3`, workerID, shiftType, date).Scan(&shift.AssignmentID, &shift.ShiftType, &shift.WorkDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	return &shift, err
}
func (r *Repository) CreateClaim(ctx context.Context, c *domain.Claim) error {
	err := r.db.QueryRow(ctx, `INSERT INTO meal_claims(worker_id,shift_assignment_id,meal_type,service_date,claimed_at,notes) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,consumed,created_at,updated_at`, c.WorkerID, c.ShiftAssignmentID, c.MealType, c.ServiceDate, c.ClaimedAt, c.Notes).Scan(&c.ID, &c.Consumed, &c.CreatedAt, &c.UpdatedAt)
	return translate(err)
}
func scanClaim(row pgx.Row) (*domain.Claim, error) {
	var c domain.Claim
	err := row.Scan(&c.ID, &c.WorkerID, &c.ShiftAssignmentID, &c.MealType, &c.ServiceDate, &c.ClaimedAt, &c.Consumed, &c.ConsumedAt, &c.ConsumptionRegisteredBy, &c.Notes, &c.CreatedAt, &c.UpdatedAt)
	return &c, translate(err)
}
func (r *Repository) FindClaim(ctx context.Context, workerID string, mealType domain.MealType, date time.Time) (*domain.Claim, error) {
	return scanClaim(r.db.QueryRow(ctx, `SELECT id,worker_id,shift_assignment_id,meal_type,service_date,claimed_at,consumed,consumed_at,consumption_registered_by,notes,created_at,updated_at FROM meal_claims WHERE worker_id=$1 AND meal_type=$2 AND service_date=$3`, workerID, mealType, date))
}
func (r *Repository) MarkConsumed(ctx context.Context, id, by string, at time.Time) (*domain.Claim, error) {
	return scanClaim(r.db.QueryRow(ctx, `UPDATE meal_claims SET consumed=TRUE,consumed_at=$2,consumption_registered_by=$3,updated_at=NOW() WHERE id=$1 AND consumed=FALSE RETURNING id,worker_id,shift_assignment_id,meal_type,service_date,claimed_at,consumed,consumed_at,consumption_registered_by,notes,created_at,updated_at`, id, at, by))
}
func (r *Repository) Report(ctx context.Context, from, to time.Time) ([]domain.ReportRow, error) {
	rows, err := r.db.Query(ctx, `WITH eligible AS (
		SELECT worker_id, work_date AS service_date, 'DESAYUNO'::meal_type AS meal_type FROM worker_shift_assignments WHERE shift_type='DIA'
		UNION
		SELECT worker_id, work_date, 'TARDE'::meal_type FROM worker_shift_assignments WHERE shift_type='DIA'
		UNION
		SELECT worker_id, work_date, 'NOCHE'::meal_type FROM worker_shift_assignments WHERE shift_type='NOCHE'
		UNION
		SELECT worker_id, work_date + 1, 'DESAYUNO'::meal_type FROM worker_shift_assignments WHERE shift_type='NOCHE'
	), types(meal_type) AS (VALUES ('DESAYUNO'::meal_type),('TARDE'::meal_type),('NOCHE'::meal_type))
	SELECT t.meal_type,
		COUNT(e.worker_id), COUNT(c.id), COUNT(c.id) FILTER(WHERE c.consumed),
		COUNT(c.id) FILTER(WHERE NOT c.consumed), COUNT(e.worker_id)-COUNT(c.id)
	FROM types t LEFT JOIN eligible e ON e.meal_type=t.meal_type AND e.service_date BETWEEN $1 AND $2
	LEFT JOIN meal_claims c ON c.worker_id=e.worker_id AND c.meal_type=e.meal_type AND c.service_date=e.service_date
	GROUP BY t.meal_type ORDER BY t.meal_type`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.ReportRow, 0)
	for rows.Next() {
		var item domain.ReportRow
		if err = rows.Scan(&item.MealType, &item.Eligible, &item.Claimed, &item.Consumed, &item.NotConsumed, &item.NotClaimed); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
