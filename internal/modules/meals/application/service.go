package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"backend/internal/modules/meals/domain"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service { return &Service{repo: repo, now: time.Now} }

func (s *Service) Claim(ctx context.Context, workerID string, mealType domain.MealType, notes string) (*domain.Claim, error) {
	if !mealType.Valid() {
		return nil, fmt.Errorf("meal_type must be DESAYUNO, TARDE or NOCHE")
	}
	rule, err := s.repo.FindRule(ctx, mealType)
	if err != nil || !rule.Active {
		return nil, fmt.Errorf("meal service is not available")
	}
	location, err := time.LoadLocation(rule.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid meal timezone configuration")
	}
	now := s.now().In(location)
	start, err := parseRuleTime(now, rule.ClaimStart)
	if err != nil {
		return nil, err
	}
	end, err := parseRuleTime(now, rule.ClaimEnd)
	if err != nil {
		return nil, err
	}
	if now.Before(start) || !now.Before(end) {
		return nil, fmt.Errorf("%s can only be claimed from %s to %s (%s)", mealType, start.Format("15:04"), end.Format("15:04"), rule.Timezone)
	}
	serviceDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	assignmentID, err := s.repo.FindEligibleAssignment(ctx, workerID, mealType, serviceDate)
	if err != nil {
		return nil, fmt.Errorf("worker is not eligible for %s on this date", mealType)
	}
	claim := &domain.Claim{WorkerID: workerID, ShiftAssignmentID: assignmentID, MealType: mealType, ServiceDate: serviceDate, ClaimedAt: now, Notes: optional(notes)}
	if err = s.repo.CreateClaim(ctx, claim); err != nil {
		return nil, err
	}
	return claim, nil
}

func parseRuleTime(date time.Time, value string) (time.Time, error) {
	parsed, err := time.Parse("15:04:05", value)
	if err != nil {
		parsed, err = time.Parse("15:04", value)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid meal window configuration")
	}
	return time.Date(date.Year(), date.Month(), date.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, date.Location()), nil
}

func optional(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
func (s *Service) MarkConsumed(ctx context.Context, claimID, registeredBy string) (*domain.Claim, error) {
	return s.repo.MarkConsumed(ctx, claimID, registeredBy, s.now())
}
func (s *Service) Report(ctx context.Context, from, to time.Time) ([]domain.ReportRow, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, fmt.Errorf("valid from and to dates are required")
	}
	return s.repo.Report(ctx, from, to)
}
