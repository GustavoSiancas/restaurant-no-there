package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	core "backend/internal/core/domain"
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

func (s *Service) WorkerStatus(ctx context.Context, workerID string) (*domain.WorkerStatus, error) {
	location, err := time.LoadLocation("America/Lima")
	if err != nil {
		location = time.FixedZone("America/Lima", -5*60*60)
	}
	now := s.now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	status := &domain.WorkerStatus{PeruTime: now}
	minutes := now.Hour()*60 + now.Minute()
	shiftType := ""
	shiftDate := today
	if minutes >= 8*60 && minutes < 17*60 {
		shiftType = "DIA"
	} else if minutes >= 20*60 {
		shiftType = "NOCHE"
	} else if minutes < 5*60 {
		shiftType = "NOCHE"
		shiftDate = today.AddDate(0, 0, -1)
	}
	if shiftType != "" {
		shift, findErr := s.repo.FindCurrentShift(ctx, workerID, shiftType, shiftDate)
		if findErr == nil {
			status.OnShift = true
			status.CurrentShift = shift
		} else if !errors.Is(findErr, core.ErrNotFound) {
			return nil, findErr
		}
	}
	rules, err := s.repo.ListRules(ctx)
	if err != nil {
		return nil, err
	}
	for _, rule := range rules {
		start, parseErr := parseRuleTime(now, rule.ClaimStart)
		if parseErr != nil {
			return nil, parseErr
		}
		end, parseErr := parseRuleTime(now, rule.ClaimEnd)
		if parseErr != nil {
			return nil, parseErr
		}
		if now.Before(start) || !now.Before(end) {
			continue
		}
		meal := &domain.CurrentMeal{MealType: rule.MealType, WindowStart: start.Format("15:04"), WindowEnd: end.Format("15:04")}
		status.MealWindowOpen = true
		status.CurrentMeal = meal
		_, eligibleErr := s.repo.FindEligibleAssignment(ctx, workerID, rule.MealType, today)
		if eligibleErr != nil {
			if errors.Is(eligibleErr, core.ErrNotFound) {
				return status, nil
			}
			return nil, eligibleErr
		}
		meal.Eligible = true
		claim, claimErr := s.repo.FindClaim(ctx, workerID, rule.MealType, today)
		if claimErr == nil {
			meal.AlreadyClaimed = true
			meal.Consumed = claim.Consumed
			meal.ClaimID = &claim.ID
		} else if errors.Is(claimErr, core.ErrNotFound) {
			meal.CanClaim = true
		} else {
			return nil, claimErr
		}
		return status, nil
	}
	return status, nil
}
