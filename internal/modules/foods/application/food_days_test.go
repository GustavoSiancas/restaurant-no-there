package application

import (
	core "backend/internal/core/domain"
	"backend/internal/modules/foods/domain"
	meals "backend/internal/modules/meals/domain"
	"context"
	"errors"
	"testing"
	"time"
)

type dayRepo struct {
	created  *domain.FoodDay
	cutoff   time.Time
	items    []domain.FoodDayItem
	date, id string
	err      error
}

func (r *dayRepo) CreateFoodDay(_ context.Context, day *domain.FoodDay) error {
	r.created = day
	return r.err
}
func (r *dayRepo) ListFoodDays(_ context.Context, date string) ([]domain.FoodDayItem, error) {
	r.date = date
	return r.items, r.err
}
func (r *dayRepo) DeleteFoodDay(_ context.Context, id string) error { r.id = id; return r.err }
func (r *dayRepo) CloseFoodDays(_ context.Context, cutoff time.Time) (int64, error) {
	r.cutoff = cutoff
	return 1, r.err
}

func TestFoodDayCreationCalendarBoundary(t *testing.T) {
	// UTC is already September 8; Lima is still September 7.
	now := time.Date(2026, 9, 8, 4, 59, 0, 0, time.UTC)
	for _, tc := range []struct {
		date  string
		valid bool
	}{{"2026-09-09", false}, {"2026-09-10", true}, {"2026-12-31", true}, {"2026-09-06", false}, {"invalid", false}} {
		r := &dayRepo{}
		s := NewFoodDayService(r, func() time.Time { return now })
		_, err := s.Create(context.Background(), CreateFoodDayInput{ServiceDate: tc.date, MealType: meals.Lunch, FoodID: "00000000-0000-0000-0000-000000000001"})
		if (err == nil) != tc.valid {
			t.Fatalf("%s: %v", tc.date, err)
		}
		if tc.valid && r.created.Status != domain.FoodDayOpen {
			t.Fatal("must create OPEN")
		}
		if !tc.valid && r.created != nil {
			t.Fatal("invalid date persisted")
		}
	}
}

func TestFoodDaySchedulerCutoff(t *testing.T) {
	for _, tc := range []struct{ now, want string }{
		{"2026-09-08T04:59:00Z", "2026-09-10"},
		{"2026-09-08T05:00:00Z", "2026-09-11"},
		{"2026-12-30T12:00:00Z", "2027-01-02"},
	} {
		now, _ := time.Parse(time.RFC3339, tc.now)
		r := &dayRepo{}
		s := NewFoodDayService(r, func() time.Time { return now })
		if _, err := s.CloseDue(context.Background()); err != nil {
			t.Fatal(err)
		}
		if r.cutoff.Format("2006-01-02") != tc.want {
			t.Fatalf("cutoff %s, want %s", r.cutoff, tc.want)
		}
	}
}

func TestFoodDayGrouping(t *testing.T) {
	r := &dayRepo{items: []domain.FoodDayItem{
		{ServiceDate: "2026-09-11", MealType: meals.Dinner, Food: domain.Food{TotalCalories: 100}},
		{ServiceDate: "2026-09-10", MealType: meals.Lunch, Food: domain.Food{TotalCalories: 200.5}},
		{ServiceDate: "2026-09-10", MealType: meals.Breakfast, Food: domain.Food{TotalCalories: 50}},
		{ServiceDate: "2026-09-10", MealType: meals.Lunch, Food: domain.Food{TotalCalories: 200.5}},
	}}
	s := NewFoodDayService(r, time.Now)
	groups, err := s.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 || groups[0].ServiceDate != "2026-09-10" || groups[0].TotalCalories != 451 {
		t.Fatalf("incorrect grouping: %+v", groups)
	}
	if groups[0].Meals[1].TotalCalories != 401 || len(groups[0].Meals[1].Foods) != 2 || len(groups[0].Meals[2].Foods) != 0 {
		t.Fatal("incorrect meal grouping")
	}
	if _, err = s.List(context.Background(), "bad"); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatal("invalid filter accepted")
	}
	if _, err = s.List(context.Background(), "2026-09-10"); err != nil || r.date != "2026-09-10" {
		t.Fatal("date filter lost")
	}
	r.items = nil
	groups, err = s.List(context.Background(), "")
	if err != nil || groups == nil || len(groups) != 0 {
		t.Fatal("empty result must be []")
	}
}

func TestFoodDayValidationAndDelete(t *testing.T) {
	r := &dayRepo{}
	s := NewFoodDayService(r, func() time.Time { return time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC) })
	for _, in := range []CreateFoodDayInput{
		{ServiceDate: "2026-09-10", MealType: "BREAK", FoodID: "00000000-0000-0000-0000-000000000001"},
		{ServiceDate: "2026-09-10", MealType: meals.Lunch, FoodID: "bad"},
	} {
		if _, err := s.Create(context.Background(), in); !errors.Is(err, core.ErrInvalidInput) {
			t.Fatal("invalid input accepted")
		}
	}
	if err := s.Delete(context.Background(), "bad"); !errors.Is(err, core.ErrInvalidInput) || r.id != "" {
		t.Fatal("invalid delete accepted")
	}
	r.err = core.ErrLocked
	if err := s.Delete(context.Background(), "00000000-0000-0000-0000-000000000001"); !errors.Is(err, core.ErrLocked) {
		t.Fatal("closed error lost")
	}
}
