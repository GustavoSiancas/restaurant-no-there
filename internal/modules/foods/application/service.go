package application

import (
	core "backend/internal/core/domain"
	"backend/internal/modules/foods/domain"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"math"
	"strings"
	"unicode/utf8"
)

type Repository interface {
	GetFood(context.Context, string) (*domain.Food, error)
	UpdateFood(context.Context, string, UpdateFoodInput) (*domain.Food, error)
	CreateFood(context.Context, *domain.Food, []string) error
	ListFoods(context.Context, int, int, []string) (*FoodPage, error)
	CreateTag(context.Context, *domain.Tag) error
	ListTags(context.Context) ([]domain.Tag, error)
}
type FoodPage struct {
	Data       []domain.Food `json:"data"`
	Page       int           `json:"page"`
	PageSize   int           `json:"page_size"`
	Total      int64         `json:"total"`
	TotalPages int64         `json:"total_pages"`
}
type CreateFoodInput struct {
	Name            string   `json:"name"`
	LongDescription string   `json:"long_description"`
	TotalCalories   *float64 `json:"total_calories"`
	PhotoURL        string   `json:"photo_url"`
	TagIDs          []string `json:"tag_ids"`
}
type Service struct{ repo Repository }

type UpdateFoodInput struct {
	Name            *string  `json:"name"`
	LongDescription *string  `json:"long_description"`
	TotalCalories   *float64 `json:"total_calories"`
	PhotoURL        *string  `json:"photo_url"`
	TagIDs          []string `json:"tag_ids"`
}

func foodID(id string) (string, error) {
	var uuid pgtype.UUID
	if err := uuid.Scan(strings.TrimSpace(id)); err != nil || !uuid.Valid {
		return "", invalid("food id must be a UUID")
	}
	value, _ := uuid.Value()
	return value.(string), nil
}

func (s *Service) GetFood(ctx context.Context, id string) (*domain.Food, error) {
	id, err := foodID(id)
	if err != nil {
		return nil, err
	}
	return s.repo.GetFood(ctx, id)
}

func (s *Service) UpdateFood(ctx context.Context, id string, in UpdateFoodInput) (*domain.Food, error) {
	id, err := foodID(id)
	if err != nil {
		return nil, err
	}
	for _, field := range []*string{in.Name, in.LongDescription, in.PhotoURL} {
		if field != nil {
			*field = strings.TrimSpace(*field)
			if *field == "" {
				return nil, invalid("provided text fields must not be blank")
			}
		}
	}
	if in.Name != nil && utf8.RuneCountInString(*in.Name) > 200 {
		return nil, invalid("name must be at most 200 characters")
	}
	if in.TotalCalories != nil && (math.IsNaN(*in.TotalCalories) || math.IsInf(*in.TotalCalories, 0) || *in.TotalCalories < 0) {
		return nil, invalid("total_calories must be a non-negative number")
	}
	if in.TagIDs != nil {
		in.TagIDs, err = normalizeIDs(in.TagIDs)
		if err != nil {
			return nil, err
		}
	}
	return s.repo.UpdateFood(ctx, id, in)
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }
func invalid(message string) error        { return fmt.Errorf("%w: %s", core.ErrInvalidInput, message) }
func normalizeIDs(ids []string) ([]string, error) {
	result := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		var uuid pgtype.UUID
		if err := uuid.Scan(strings.TrimSpace(id)); err != nil || !uuid.Valid {
			return nil, invalid("tag_ids must contain UUIDs")
		}
		value, _ := uuid.Value()
		normalized := value.(string)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	return result, nil
}
func (s *Service) CreateFood(ctx context.Context, in CreateFoodInput) (*domain.Food, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.LongDescription = strings.TrimSpace(in.LongDescription)
	in.PhotoURL = strings.TrimSpace(in.PhotoURL)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 200 || in.LongDescription == "" || in.PhotoURL == "" {
		return nil, invalid("name (max 200 characters), long_description and photo_url are required")
	}
	if in.TotalCalories == nil || math.IsNaN(*in.TotalCalories) || math.IsInf(*in.TotalCalories, 0) || *in.TotalCalories < 0 {
		return nil, invalid("total_calories must be a non-negative number")
	}
	ids, err := normalizeIDs(in.TagIDs)
	if err != nil {
		return nil, err
	}
	food := &domain.Food{Name: in.Name, LongDescription: in.LongDescription, TotalCalories: *in.TotalCalories, PhotoURL: in.PhotoURL, Tags: []domain.Tag{}}
	if err = s.repo.CreateFood(ctx, food, ids); err != nil {
		return nil, err
	}
	return food, nil
}
func (s *Service) ListFoods(ctx context.Context, page, size int, ids []string) (*FoodPage, error) {
	if page < 1 || size < 1 || size > 100 || page > math.MaxInt/size {
		return nil, invalid("page must be positive and page_size must be between 1 and 100")
	}
	ids, err := normalizeIDs(ids)
	if err != nil {
		return nil, err
	}
	return s.repo.ListFoods(ctx, page, size, ids)
}
func (s *Service) CreateTag(ctx context.Context, name string) (*domain.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		return nil, invalid("name is required and must be at most 100 characters")
	}
	tag := &domain.Tag{Name: name}
	if err := s.repo.CreateTag(ctx, tag); err != nil {
		return nil, err
	}
	return tag, nil
}
func (s *Service) ListTags(ctx context.Context) ([]domain.Tag, error) { return s.repo.ListTags(ctx) }
