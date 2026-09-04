package application

import (
	"backend/internal/modules/foods/domain"
	mealsdomain "backend/internal/modules/meals/domain"
	"context"
	"sort"
	"time"
)

type FoodDayRepository interface {
	CreateFoodDay(context.Context, *domain.FoodDay) error
	ListFoodDays(context.Context, string) ([]domain.FoodDayItem, error)
	DeleteFoodDay(context.Context, string) error
	CloseFoodDays(context.Context, time.Time) (int64, error)
}

type FoodDayService struct {
	repo     FoodDayRepository
	now      func() time.Time
	location *time.Location
}

func NewFoodDayService(repo FoodDayRepository, now func() time.Time) *FoodDayService {
	location, err := time.LoadLocation("America/Lima")
	if err != nil {
		location = time.FixedZone("America/Lima", -5*60*60)
	}
	return &FoodDayService{repo: repo, now: now, location: location}
}

type CreateFoodDayInput struct {
	ServiceDate string               `json:"service_date"`
	MealType    mealsdomain.MealType `json:"meal_type"`
	FoodID      string               `json:"food_id"`
}

func (s *FoodDayService) today() time.Time {
	now := s.now().In(s.location)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.location)
}

func (s *FoodDayService) Create(ctx context.Context, in CreateFoodDayInput) (*domain.FoodDay, error) {
	date, err := time.ParseInLocation("2006-01-02", in.ServiceDate, s.location)
	if err != nil {
		return nil, invalid("service_date must use YYYY-MM-DD")
	}
	if date.Before(s.today().AddDate(0, 0, 3)) {
		return nil, invalid("food days must be created at least 3 calendar days before service_date (America/Lima)")
	}
	if !in.MealType.Valid() {
		return nil, invalid("meal_type must be BREAKFAST, LUNCH or DINNER")
	}
	id, err := foodID(in.FoodID)
	if err != nil {
		return nil, err
	}
	day := &domain.FoodDay{ServiceDate: date, MealType: in.MealType, FoodID: id, Status: domain.FoodDayOpen}
	if err = s.repo.CreateFoodDay(ctx, day); err != nil {
		return nil, err
	}
	return day, nil
}

func (s *FoodDayService) List(ctx context.Context, date string) ([]domain.FoodDayGroup, error) {
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return nil, invalid("date must use YYYY-MM-DD")
		}
	}
	items, err := s.repo.ListFoodDays(ctx, date)
	if err != nil {
		return nil, err
	}
	groups := []domain.FoodDayGroup{}
	indexes := map[string]int{}
	for _, item := range items {
		index, ok := indexes[item.ServiceDate]
		if !ok {
			index = len(groups)
			indexes[item.ServiceDate] = index
			groups = append(groups, domain.FoodDayGroup{ServiceDate: item.ServiceDate, Meals: []domain.FoodDayMeal{
				{MealType: mealsdomain.Breakfast, Foods: []domain.FoodDayItem{}},
				{MealType: mealsdomain.Lunch, Foods: []domain.FoodDayItem{}},
				{MealType: mealsdomain.Dinner, Foods: []domain.FoodDayItem{}},
			}})
		}
		group := &groups[index]
		for i := range group.Meals {
			if group.Meals[i].MealType == item.MealType {
				group.Meals[i].Foods = append(group.Meals[i].Foods, item)
				group.Meals[i].TotalCalories += item.Food.TotalCalories
				group.TotalCalories += item.Food.TotalCalories
				break
			}
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].ServiceDate < groups[j].ServiceDate })
	return groups, nil
}

func (s *FoodDayService) Delete(ctx context.Context, id string) error {
	normalized, err := foodID(id)
	if err != nil {
		return invalid("food day id must be a UUID")
	}
	return s.repo.DeleteFoodDay(ctx, normalized)
}

// CloseDue closes at the beginning of the day three calendar days before service.
// Including older dates catches up after downtime; repeated calls are idempotent.
func (s *FoodDayService) CloseDue(ctx context.Context) (int64, error) {
	return s.repo.CloseFoodDays(ctx, s.today().AddDate(0, 0, 3))
}
