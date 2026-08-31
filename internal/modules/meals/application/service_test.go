package application

import (
	"context"
	"strings"
	"testing"
	"time"

	core "backend/internal/core/domain"
	"backend/internal/modules/meals/domain"
)

type fakeMealsRepository struct {
	created  bool
	eligible bool
	rules    []domain.ServiceRule
	claim    *domain.Claim
}

func (f *fakeMealsRepository) FindRule(context.Context, domain.MealType) (*domain.ServiceRule, error) {
	return &domain.ServiceRule{ClaimStart: "06:00:00", ClaimEnd: "10:00:00", Timezone: "America/Lima", Active: true}, nil
}
func (f *fakeMealsRepository) ListRules(context.Context) ([]domain.ServiceRule, error) {
	return f.rules, nil
}
func (f *fakeMealsRepository) FindCurrentShift(context.Context, string, string, time.Time) (*domain.CurrentShift, error) {
	return nil, core.ErrNotFound
}
func (f *fakeMealsRepository) FindClaim(context.Context, string, domain.MealType, time.Time) (*domain.Claim, error) {
	if f.claim != nil {
		return f.claim, nil
	}
	return nil, core.ErrNotFound
}
func (f *fakeMealsRepository) FindEligibleAssignment(context.Context, string, domain.MealType, time.Time) (string, error) {
	if !f.eligible {
		return "", core.ErrNotFound
	}
	return "assignment", nil
}
func (f *fakeMealsRepository) CreateClaim(_ context.Context, c *domain.Claim) error {
	f.created = true
	c.ID = "claim"
	return nil
}
func (f *fakeMealsRepository) Report(context.Context, time.Time, time.Time) ([]domain.ReportRow, error) {
	return nil, nil
}

func TestClaimUsesPeruWindowAndEligibleShift(t *testing.T) {
	repo := &fakeMealsRepository{eligible: true}
	service := NewService(repo)
	service.now = func() time.Time { return time.Date(2026, 9, 1, 8, 0, 0, 0, time.FixedZone("UTC-5", -5*60*60)) }
	claim, err := service.Claim(context.Background(), "worker", domain.Breakfast, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claim.ID != "claim" || !repo.created {
		t.Fatal("eligible claim was not created")
	}
}

func TestClaimRejectsOutsideMealWindow(t *testing.T) {
	repo := &fakeMealsRepository{eligible: true}
	service := NewService(repo)
	service.now = func() time.Time { return time.Date(2026, 9, 1, 10, 0, 0, 0, time.FixedZone("UTC-5", -5*60*60)) }
	_, err := service.Claim(context.Background(), "worker", domain.Breakfast, "")
	if err == nil || !strings.Contains(err.Error(), "can only be claimed") {
		t.Fatalf("expected window error, got %v", err)
	}
	if repo.created {
		t.Fatal("claim outside the window must not be created")
	}
}

func TestClaimRejectsWorkerWithoutEligibleShift(t *testing.T) {
	repo := &fakeMealsRepository{}
	service := NewService(repo)
	service.now = func() time.Time { return time.Date(2026, 9, 1, 8, 0, 0, 0, time.FixedZone("UTC-5", -5*60*60)) }
	_, err := service.Claim(context.Background(), "worker", domain.Breakfast, "")
	if err == nil || !strings.Contains(err.Error(), "not eligible") {
		t.Fatalf("expected eligibility error, got %v", err)
	}
}

func TestWorkerStatusShowsAvailableUnclaimedBreakfast(t *testing.T) {
	repo := &fakeMealsRepository{eligible: true, rules: []domain.ServiceRule{{MealType: domain.Breakfast, ClaimStart: "06:00:00", ClaimEnd: "10:00:00", Timezone: "America/Lima", Active: true}}}
	service := NewService(repo)
	service.now = func() time.Time { return time.Date(2026, 9, 2, 6, 30, 0, 0, time.FixedZone("UTC-5", -5*60*60)) }
	status, err := service.WorkerStatus(context.Background(), "worker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.OnShift {
		t.Fatal("06:30 is outside the fixed active shift hours")
	}
	if status.CurrentMeal == nil || !status.CurrentMeal.Eligible || !status.CurrentMeal.CanClaim || status.CurrentMeal.AlreadyClaimed {
		t.Fatalf("unexpected meal status: %+v", status.CurrentMeal)
	}
}
