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
}

func (f *fakeMealsRepository) FindRule(context.Context, domain.MealType) (*domain.ServiceRule, error) {
	return &domain.ServiceRule{ClaimStart: "06:00:00", ClaimEnd: "10:00:00", Timezone: "America/Lima", Active: true}, nil
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
func (f *fakeMealsRepository) MarkConsumed(context.Context, string, string, time.Time) (*domain.Claim, error) {
	return nil, nil
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
