package postgres

import (
	"context"
	"errors"
	"fmt"
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
		)
		ORDER BY CASE WHEN a.shift_type='NOCHE' THEN 0 ELSE 1 END
		LIMIT 1`, workerID, mealType, date).Scan(&id)
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
	err := r.db.QueryRow(ctx, `INSERT INTO meal_claims(worker_id,shift_assignment_id,meal_type,service_date,claimed_at,notes) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,status,validated_at,validated_by,created_at,updated_at`, c.WorkerID, c.ShiftAssignmentID, c.MealType, c.ServiceDate, c.ClaimedAt, c.Notes).Scan(&c.ID, &c.Status, &c.ValidatedAt, &c.ValidatedBy, &c.CreatedAt, &c.UpdatedAt)
	return translate(err)
}
func scanClaim(row pgx.Row) (*domain.Claim, error) {
	var c domain.Claim
	err := row.Scan(&c.ID, &c.WorkerID, &c.ShiftAssignmentID, &c.MealType, &c.ServiceDate, &c.ClaimedAt, &c.Status, &c.ValidatedAt, &c.ValidatedBy, &c.Notes, &c.CreatedAt, &c.UpdatedAt)
	return &c, translate(err)
}
func (r *Repository) FindClaim(ctx context.Context, workerID string, mealType domain.MealType, date time.Time) (*domain.Claim, error) {
	return scanClaim(r.db.QueryRow(ctx, `SELECT id,worker_id,shift_assignment_id,meal_type,service_date,claimed_at,status,validated_at,validated_by,notes,created_at,updated_at FROM meal_claims WHERE worker_id=$1 AND meal_type=$2 AND service_date=$3`, workerID, mealType, date))
}
func (r *Repository) FindWorkerTicketIdentity(ctx context.Context, workerID string) (*domain.WorkerTicketIdentity, error) {
	var identity domain.WorkerTicketIdentity
	err := r.db.QueryRow(ctx, `SELECT u.id,p.first_name,p.last_name,c.identifier
		FROM users u
		JOIN user_profiles p ON p.user_id=u.id
		JOIN user_credentials c ON c.user_id=u.id AND c.type='DNI' AND c.active=TRUE
		WHERE u.id=$1 AND u.role='WORKER' AND u.active=TRUE`, workerID).
		Scan(&identity.ID, &identity.FirstName, &identity.LastName, &identity.DNI)
	return &identity, translate(err)
}

const orderColumns = `c.id,c.worker_id,c.shift_assignment_id,c.meal_type,c.service_date,c.claimed_at,c.status,c.validated_at,c.validated_by,c.notes,c.created_at,c.updated_at,
	u.id,BTRIM(p.first_name || ' ' || p.last_name),REPEAT('*',GREATEST(LENGTH(dni.identifier)-4,0)) || RIGHT(dni.identifier,4)`

func scanOrder(row pgx.Row) (*domain.MealOrder, error) {
	var order domain.MealOrder
	err := row.Scan(&order.ID, &order.WorkerID, &order.ShiftAssignmentID, &order.MealType, &order.ServiceDate, &order.ClaimedAt, &order.Status, &order.ValidatedAt, &order.ValidatedBy, &order.Notes, &order.CreatedAt, &order.UpdatedAt, &order.Worker.ID, &order.Worker.FullName, &order.Worker.DocumentNumber)
	if err != nil {
		return nil, translate(err)
	}
	order.Service = orderService(order.MealType)
	return &order, nil
}

func orderService(mealType domain.MealType) domain.ClaimPreviewService {
	switch mealType {
	case domain.Breakfast:
		return domain.ClaimPreviewService{Type: "BREAKFAST", Name: "DESAYUNO"}
	case domain.Afternoon:
		return domain.ClaimPreviewService{Type: "LUNCH", Name: "ALMUERZO"}
	default:
		return domain.ClaimPreviewService{Type: "DINNER", Name: "CENA"}
	}
}

const orderJoins = ` JOIN users u ON u.id=c.worker_id
	JOIN user_profiles p ON p.user_id=u.id
	JOIN user_credentials dni ON dni.user_id=u.id AND dni.type='DNI' AND dni.active=TRUE `

func (r *Repository) ListOrders(ctx context.Context, status domain.ClaimStatus) ([]domain.MealOrder, error) {
	rows, err := r.db.Query(ctx, `SELECT `+orderColumns+` FROM meal_claims c `+orderJoins+` WHERE c.status=$1 ORDER BY c.created_at`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := make([]domain.MealOrder, 0)
	for rows.Next() {
		order, scanErr := scanOrder(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		orders = append(orders, *order)
	}
	return orders, rows.Err()
}

func (r *Repository) FindOrderByID(ctx context.Context, id string) (*domain.MealOrder, error) {
	return scanOrder(r.db.QueryRow(ctx, `SELECT `+orderColumns+` FROM meal_claims c `+orderJoins+` WHERE c.id=$1`, id))
}

func (r *Repository) ValidateOrder(ctx context.Context, id, validatedBy string, validatedAt time.Time) (*domain.MealOrder, error) {
	return scanOrder(r.db.QueryRow(ctx, `WITH updated AS (
		UPDATE meal_claims c SET status='VALIDATED',validated_at=$2,validated_by=$3,updated_at=NOW()
		FROM meal_service_rules r
		WHERE c.id=$1 AND c.status='REQUESTED' AND r.meal_type=c.meal_type
		  AND $2 >= ((c.service_date + r.claim_start) AT TIME ZONE r.timezone)
		  AND $2 < ((c.service_date + r.claim_end) AT TIME ZONE r.timezone)
		RETURNING c.*
	) SELECT `+orderColumns+` FROM updated c `+orderJoins, id, validatedAt, validatedBy))
}

func (r *Repository) EarliestPendingServiceDate(ctx context.Context) (*time.Time, error) {
	var date *time.Time
	err := r.db.QueryRow(ctx, `WITH eligible AS (
		SELECT worker_id,work_date AS service_date,'DESAYUNO'::meal_type meal_type FROM worker_shift_assignments WHERE shift_type='DIA'
		UNION SELECT worker_id,work_date,'TARDE'::meal_type FROM worker_shift_assignments WHERE shift_type='DIA'
		UNION SELECT worker_id,work_date,'NOCHE'::meal_type FROM worker_shift_assignments WHERE shift_type='NOCHE'
		UNION SELECT worker_id,work_date+1,'DESAYUNO'::meal_type FROM worker_shift_assignments WHERE shift_type='NOCHE'
	) SELECT MIN(e.service_date) FROM eligible e
	LEFT JOIN meal_claims c ON c.worker_id=e.worker_id AND c.meal_type=e.meal_type AND c.service_date=e.service_date
	WHERE c.id IS NULL OR c.status='REQUESTED'`).Scan(&date)
	return date, err
}

func (r *Repository) CloseMealWindow(ctx context.Context, mealType domain.MealType, serviceDate, closedAt time.Time) (domain.MealWindowClosure, error) {
	var result domain.MealWindowClosure
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	inserted, err := tx.Exec(ctx, `INSERT INTO meal_claims
		(worker_id,shift_assignment_id,meal_type,service_date,claimed_at,status,notes,created_at,updated_at)
		SELECT a.worker_id,a.id,$1::meal_type,$2::date,NULL,'NOT_CONSUMED','Generado automáticamente al cerrar el horario de comida',$3::timestamptz,$3::timestamptz
		FROM worker_shift_assignments a
		WHERE (
			($1::meal_type='DESAYUNO' AND ((a.shift_type='DIA' AND a.work_date=$2::date) OR (a.shift_type='NOCHE' AND a.work_date=($2::date - 1))))
			OR ($1::meal_type='TARDE' AND a.shift_type='DIA' AND a.work_date=$2::date)
			OR ($1::meal_type='NOCHE' AND a.shift_type='NOCHE' AND a.work_date=$2::date)
		)
		ON CONFLICT (worker_id,meal_type,service_date) DO NOTHING`, mealType, serviceDate, closedAt)
	if err != nil {
		return result, err
	}
	result.NotConsumed = inserted.RowsAffected()
	updated, err := tx.Exec(ctx, `UPDATE meal_claims
		SET status='REQUESTED_BUT_NOT_VALIDATED',updated_at=$3::timestamptz
		WHERE meal_type=$1::meal_type AND service_date=$2::date AND status='REQUESTED'`, mealType, serviceDate, closedAt)
	if err != nil {
		return result, err
	}
	result.RequestedNotValidated = updated.RowsAffected()
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

const detailedReportWhere = ` FROM meal_claims c
	JOIN worker_shift_assignments a ON a.id=c.shift_assignment_id
	JOIN user_profiles p ON p.user_id=c.worker_id
	JOIN user_credentials dni ON dni.user_id=c.worker_id AND dni.type='DNI' AND dni.active=TRUE
	JOIN worker_information wi ON wi.user_id=c.worker_id
	WHERE c.service_date BETWEEN $1::date AND $2::date
	AND (NULLIF($3,'') IS NULL OR c.meal_type=NULLIF($3,'')::meal_type)
	AND (NULLIF($4,'') IS NULL OR a.shift_type=NULLIF($4,'')::shift_type)`

func (r *Repository) DetailedReportSummary(ctx context.Context, filters domain.ReportFilters) (domain.DetailedReportSummary, error) {
	var summary domain.DetailedReportSummary
	err := r.db.QueryRow(ctx, `SELECT COUNT(*),
		COUNT(*) FILTER(WHERE c.status='VALIDATED'),
		COUNT(*) FILTER(WHERE c.status='REQUESTED_BUT_NOT_VALIDATED'),
		COUNT(*) FILTER(WHERE c.status='NOT_CONSUMED'),
		COUNT(*) FILTER(WHERE c.status IN ('REQUESTED_BUT_NOT_VALIDATED','NOT_CONSUMED'))`+detailedReportWhere,
		filters.From, filters.To, filters.MealType, filters.ShiftType).
		Scan(&summary.TotalEligible, &summary.Consumed, &summary.RequestedNotValidated, &summary.NotClaimed, &summary.DidNotConsume)
	return summary, err
}

func (r *Repository) DetailedReportRows(ctx context.Context, filters domain.ReportFilters, limit, offset int) ([]domain.DetailedReportRow, error) {
	query := `SELECT c.id,c.service_date,c.meal_type,a.shift_type,c.status,c.claimed_at,c.validated_at,c.worker_id,
		BTRIM(p.first_name || ' ' || p.last_name),
		REPEAT('*',GREATEST(LENGTH(dni.identifier)-4,0)) || RIGHT(dni.identifier,4),
		wi.employee_code,wi.department` + detailedReportWhere + ` ORDER BY c.service_date DESC,c.meal_type,c.created_at`
	args := []any{filters.From, filters.To, filters.MealType, filters.ShiftType}
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
		args = append(args, limit, offset)
	}
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.DetailedReportRow, 0)
	for rows.Next() {
		var row domain.DetailedReportRow
		if err = rows.Scan(&row.ID, &row.ServiceDate, &row.MealType, &row.ShiftType, &row.Status, &row.ClaimedAt, &row.ValidatedAt, &row.WorkerID, &row.FullName, &row.DocumentNumber, &row.EmployeeCode, &row.Department); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
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
	SELECT t.meal_type, COUNT(e.worker_id),
		COUNT(c.id) FILTER(WHERE c.status IN ('REQUESTED','VALIDATED','REQUESTED_BUT_NOT_VALIDATED')),
		COUNT(e.worker_id)-COUNT(c.id) FILTER(WHERE c.status IN ('REQUESTED','VALIDATED','REQUESTED_BUT_NOT_VALIDATED'))
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
		if err = rows.Scan(&item.MealType, &item.Eligible, &item.Claimed, &item.NotClaimed); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
