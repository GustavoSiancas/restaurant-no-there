package postgres

import (
	core "backend/internal/core/domain"
	"backend/internal/modules/foods/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"time"
)

func (r *Repository) CreateFoodDay(ctx context.Context, day *domain.FoodDay) error {
	err := r.db.QueryRow(ctx, `INSERT INTO food_days(service_date,meal_type,food_id,status) VALUES($1::date,$2,$3,'OPEN') RETURNING id,status,created_at,updated_at`, day.ServiceDate.Format("2006-01-02"), day.MealType, day.FoodID).Scan(&day.ID, &day.Status, &day.CreatedAt, &day.UpdatedAt)
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23503" {
		return fmt.Errorf("%w: food_id does not exist", core.ErrInvalidInput)
	}
	return err
}

func (r *Repository) ListFoodDays(ctx context.Context, date string) ([]domain.FoodDayItem, error) {
	rows, err := r.db.Query(ctx, `SELECT fd.id,to_char(fd.service_date,'YYYY-MM-DD'),fd.meal_type,fd.food_id,fd.status,
	to_jsonb(f) || jsonb_build_object('tags', COALESCE((SELECT jsonb_agg(to_jsonb(t) ORDER BY t.name,t.id) FROM tags t JOIN food_tags ft ON ft.tag_id=t.id WHERE ft.food_id=f.id),'[]'::jsonb))
	FROM food_days fd JOIN foods f ON f.id=fd.food_id
	WHERE ($1::text='' OR fd.service_date=NULLIF($1,'')::date)
	ORDER BY fd.service_date,fd.meal_type,fd.created_at,fd.id`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.FoodDayItem{}
	for rows.Next() {
		var item domain.FoodDayItem
		var food []byte
		if err = rows.Scan(&item.ID, &item.ServiceDate, &item.MealType, &item.FoodID, &item.Status, &food); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(food, &item.Food); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) DeleteFoodDay(ctx context.Context, id string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status domain.FoodDayStatus
	err = tx.QueryRow(ctx, `SELECT status FROM food_days WHERE id=$1 FOR UPDATE`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return core.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != domain.FoodDayOpen {
		return core.ErrLocked
	}
	if _, err = tx.Exec(ctx, `DELETE FROM food_days WHERE id=$1 AND status='OPEN'`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) CloseFoodDays(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.Exec(ctx, `UPDATE food_days SET status='CLOSED',updated_at=NOW() WHERE status='OPEN' AND service_date<=$1::date`, cutoff.Format("2006-01-02"))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}
