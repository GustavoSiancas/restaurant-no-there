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

func NewService(repo Repository, clocks ...func() time.Time) *Service {
	clock := time.Now
	if len(clocks) > 0 && clocks[0] != nil {
		clock = clocks[0]
	}
	return &Service{repo: repo, now: clock}
}

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
func (s *Service) Report(ctx context.Context, from, to time.Time) ([]domain.ReportRow, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, fmt.Errorf("valid from and to dates are required")
	}
	return s.repo.Report(ctx, from, to)
}

func (s *Service) ListSchedules(ctx context.Context) ([]domain.ServiceRule, error) {
	return s.repo.ListRules(ctx)
}

func (s *Service) ClaimPreview(ctx context.Context, workerID string) (*domain.ClaimPreview, error) {
	status, err := s.WorkerStatus(ctx, workerID)
	if err != nil {
		return nil, err
	}
	identity, err := s.repo.FindWorkerTicketIdentity(ctx, workerID)
	if err != nil {
		return nil, err
	}
	preview := &domain.ClaimPreview{Status: "DENIED", Date: status.PeruTime.Format("02/01/2006"), Time: formatTicketTime(status.PeruTime)}
	preview.Worker = domain.ClaimPreviewWorker{ID: identity.ID, FullName: strings.TrimSpace(identity.FirstName + " " + identity.LastName), DocumentNumber: maskDocument(identity.DNI)}
	if !status.MealWindowOpen || status.CurrentMeal == nil {
		preview.Reason = "no hay un horario de comida disponible en este momento"
		return preview, nil
	}
	if !status.CurrentMeal.Eligible {
		preview.Reason = "el trabajador no tiene un turno elegible para esta comida"
		return preview, nil
	}
	if status.CurrentMeal.AlreadyClaimed {
		preview.Reason = "la comida ya fue reclamada hoy"
		return preview, nil
	}
	if !status.CurrentMeal.CanClaim || status.CurrentShift == nil {
		preview.Reason = "la comida no está disponible para reclamar"
		return preview, nil
	}
	preview.Status = "AUTHORIZED"
	preview.RedemptionID = status.CurrentShift.AssignmentID
	preview.TicketNumber = ticketNumber(status.PeruTime, status.CurrentShift.AssignmentID, status.CurrentMeal.MealType)
	preview.Service = ticketService(status.CurrentMeal.MealType)
	return preview, nil
}

func maskDocument(document string) string {
	document = strings.TrimSpace(document)
	if len(document) <= 4 {
		return strings.Repeat("*", len(document))
	}
	return strings.Repeat("*", len(document)-4) + document[len(document)-4:]
}

func formatTicketTime(value time.Time) string {
	suffix := "a.m."
	hour := value.Hour()
	if hour >= 12 {
		suffix = "p.m."
	}
	displayHour := hour % 12
	if displayHour == 0 {
		displayHour = 12
	}
	return fmt.Sprintf("%02d:%02d %s", displayHour, value.Minute(), suffix)
}

func ticketNumber(date time.Time, assignmentID string, mealType domain.MealType) string {
	compactID := strings.ToUpper(strings.ReplaceAll(assignmentID, "-", ""))
	if len(compactID) > 8 {
		compactID = compactID[:8]
	}
	return fmt.Sprintf("TK-%s-%s-%s", date.Format("20060102"), compactID, mealType)
}

func ticketService(mealType domain.MealType) *domain.ClaimPreviewService {
	service := &domain.ClaimPreviewService{Name: string(mealType)}
	switch mealType {
	case domain.Breakfast:
		service.Type, service.Name = "BREAKFAST", "DESAYUNO"
	case domain.Afternoon:
		service.Type, service.Name = "LUNCH", "ALMUERZO"
	case domain.Night:
		service.Type, service.Name = "DINNER", "CENA"
	}
	return service
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
