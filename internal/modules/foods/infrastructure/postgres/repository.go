package postgres

import (
	core "backend/internal/core/domain"
	"backend/internal/modules/foods/application"
	"backend/internal/modules/foods/domain"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

var _ application.Repository = (*Repository)(nil)

type foodQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func getFood(ctx context.Context, db foodQuerier, id string) (*domain.Food, error) {
	var f domain.Food
	var tags []byte
	err := db.QueryRow(ctx, `SELECT f.id,f.name,f.long_description,f.total_calories,f.photo_url,f.created_at,f.updated_at,
	COALESCE((SELECT jsonb_agg(to_jsonb(t) ORDER BY t.name,t.id) FROM tags t JOIN food_tags ft ON ft.tag_id=t.id WHERE ft.food_id=f.id),'[]'::jsonb)
	FROM foods f WHERE f.id=$1`, id).Scan(&f.ID, &f.Name, &f.LongDescription, &f.TotalCalories, &f.PhotoURL, &f.CreatedAt, &f.UpdatedAt, &tags)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, core.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(tags, &f.Tags); err != nil {
		return nil, err
	}
	return &f, nil
}

func (r *Repository) GetFood(ctx context.Context, id string) (*domain.Food, error) {
	return getFood(ctx, r.db, id)
}

func (r *Repository) UpdateFood(ctx context.Context, id string, in application.UpdateFoodInput) (*domain.Food, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// Updating the parent row serializes concurrent tag replacements for this food.
	result, err := tx.Exec(ctx, `UPDATE foods SET name=COALESCE($2,name),long_description=COALESCE($3,long_description),
	total_calories=COALESCE($4,total_calories),photo_url=COALESCE($5,photo_url),updated_at=NOW() WHERE id=$1`,
		id, in.Name, in.LongDescription, in.TotalCalories, in.PhotoURL)
	if err != nil {
		return nil, err
	}
	if result.RowsAffected() == 0 {
		return nil, core.ErrNotFound
	}
	if in.TagIDs != nil {
		if _, err = tx.Exec(ctx, `DELETE FROM food_tags WHERE food_id=$1`, id); err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO food_tags(food_id,tag_id) SELECT $1::uuid,unnest($2::uuid[])`, id, in.TagIDs)
		if err != nil {
			var pgerr *pgconn.PgError
			if errors.As(err, &pgerr) && pgerr.Code == "23503" {
				return nil, fmt.Errorf("%w: one or more tag_ids do not exist", core.ErrInvalidInput)
			}
			return nil, err
		}
	}
	f, err := getFood(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return f, nil
}

func (r *Repository) CreateFood(ctx context.Context, f *domain.Food, ids []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO foods(name,long_description,total_calories,photo_url) VALUES($1,$2,$3,$4) RETURNING id,created_at,updated_at`, f.Name, f.LongDescription, f.TotalCalories, f.PhotoURL).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return err
	}
	// Lock selected tags until commit so they cannot disappear during assignment.
	rows, err := tx.Query(ctx, `SELECT id,name,created_at,updated_at FROM tags WHERE id=ANY($1::uuid[]) ORDER BY name,id FOR KEY SHARE`, ids)
	if err != nil {
		return err
	}
	f.Tags = []domain.Tag{}
	for rows.Next() {
		var t domain.Tag
		if err = rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt); err != nil {
			rows.Close()
			return err
		}
		f.Tags = append(f.Tags, t)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if len(f.Tags) != len(ids) {
		return fmt.Errorf("%w: one or more tag_ids do not exist", core.ErrInvalidInput)
	}
	_, err = tx.Exec(ctx, `INSERT INTO food_tags(food_id,tag_id) SELECT $1::uuid,unnest($2::uuid[])`, f.ID, ids)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) ListFoods(ctx context.Context, page, size int, ids []string) (*application.FoodPage, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	result := &application.FoodPage{Data: []domain.Food{}, Page: page, PageSize: size}
	const filter = ` WHERE cardinality($1::uuid[])=0 OR EXISTS (SELECT 1 FROM food_tags ft WHERE ft.food_id=f.id AND ft.tag_id=ANY($1::uuid[]))`
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM foods f`+filter, ids).Scan(&result.Total); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT f.id,f.name,f.long_description,f.total_calories,f.photo_url,f.created_at,f.updated_at,
 COALESCE((SELECT jsonb_agg(to_jsonb(t) ORDER BY t.name,t.id) FROM tags t JOIN food_tags ft ON ft.tag_id=t.id WHERE ft.food_id=f.id),'[]'::jsonb)
 FROM foods f`+filter+` ORDER BY f.created_at DESC,f.id DESC LIMIT $2 OFFSET $3`, ids, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var f domain.Food
		var tags []byte
		if err = rows.Scan(&f.ID, &f.Name, &f.LongDescription, &f.TotalCalories, &f.PhotoURL, &f.CreatedAt, &f.UpdatedAt, &tags); err != nil {
			rows.Close()
			return nil, err
		}
		if err = json.Unmarshal(tags, &f.Tags); err != nil {
			rows.Close()
			return nil, err
		}
		result.Data = append(result.Data, f)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	result.TotalPages = (result.Total + int64(size) - 1) / int64(size)
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}
func (r *Repository) CreateTag(ctx context.Context, t *domain.Tag) error {
	err := r.db.QueryRow(ctx, `INSERT INTO tags(name) VALUES($1) RETURNING id,created_at,updated_at`, t.Name).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" {
		return core.ErrConflict
	}
	return err
}
func (r *Repository) ListTags(ctx context.Context) ([]domain.Tag, error) {
	rows, err := r.db.Query(ctx, `SELECT id,name,created_at,updated_at FROM tags ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []domain.Tag{}
	for rows.Next() {
		var t domain.Tag
		if err = rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}
