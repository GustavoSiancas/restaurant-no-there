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
	claim := &domain.Claim{WorkerID: workerID, ShiftAssignmentID: assignmentID, MealType: mealType, ServiceDate: serviceDate, ClaimedAt: &now, Notes: optional(notes)}
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

func (s *Service) CloseExpiredMealWindows(ctx context.Context, lookbackDays int) (int64, error) {
	if lookbackDays < 0 {
		return 0, fmt.Errorf("lookback days cannot be negative")
	}
	now := s.now().In(s.peruLocation())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	firstDate := today.AddDate(0, 0, -lookbackDays)
	oldestPending, err := s.repo.EarliestPendingServiceDate(ctx)
	if err != nil {
		return 0, err
	}
	if oldestPending != nil {
		pendingDate := time.Date(oldestPending.Year(), oldestPending.Month(), oldestPending.Day(), 0, 0, 0, 0, now.Location())
		if pendingDate.Before(firstDate) {
			firstDate = pendingDate
		}
	}
	rules, err := s.repo.ListRules(ctx)
	if err != nil {
		return 0, err
	}
	var created int64
	for serviceDate := firstDate; !serviceDate.After(today); serviceDate = serviceDate.AddDate(0, 0, 1) {
		for _, rule := range rules {
			if !rule.Active {
				continue
			}
			windowEnd, parseErr := parseRuleTime(serviceDate, rule.ClaimEnd)
			if parseErr != nil {
				return created, parseErr
			}
			if now.Before(windowEnd) {
				continue
			}
			closure, createErr := s.repo.CloseMealWindow(ctx, rule.MealType, serviceDate, now)
			if createErr != nil {
				return created, createErr
			}
			created += closure.NotConsumed + closure.RequestedNotValidated
		}
	}
	return created, nil
}

func (s *Service) DetailedReport(ctx context.Context, filters domain.ReportFilters, page, pageSize int, paginate bool) (*domain.DetailedReport, error) {
	now := s.now().In(s.peruLocation())
	yesterday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.peruLocation()).AddDate(0, 0, -1)
	if filters.From.IsZero() && filters.To.IsZero() {
		filters.From, filters.To = yesterday, yesterday
	} else if filters.From.IsZero() {
		filters.From = filters.To
	} else if filters.To.IsZero() {
		filters.To = filters.From
	}
	filters.From = time.Date(filters.From.Year(), filters.From.Month(), filters.From.Day(), 0, 0, 0, 0, s.peruLocation())
	filters.To = time.Date(filters.To.Year(), filters.To.Month(), filters.To.Day(), 0, 0, 0, 0, s.peruLocation())
	if filters.To.Before(filters.From) {
		return nil, fmt.Errorf("to must be on or after from")
	}
	if filters.MealType != "" && !filters.MealType.Valid() {
		return nil, fmt.Errorf("meal_type must be DESAYUNO, TARDE or NOCHE")
	}
	if filters.ShiftType != "" && filters.ShiftType != "DIA" && filters.ShiftType != "NOCHE" {
		return nil, fmt.Errorf("shift_type must be DIA or NOCHE")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	summary, err := s.repo.DetailedReportSummary(ctx, filters)
	if err != nil {
		return nil, err
	}
	limit, offset := 0, 0
	if paginate {
		limit, offset = pageSize, (page-1)*pageSize
	}
	rows, err := s.repo.DetailedReportRows(ctx, filters, limit, offset)
	if err != nil {
		return nil, err
	}
	totalPages := 0
	if summary.TotalEligible > 0 {
		totalPages = int((summary.TotalEligible + int64(pageSize) - 1) / int64(pageSize))
	}
	return &domain.DetailedReport{Filters: filters, Summary: summary, Data: rows, Page: page, PageSize: pageSize, Total: summary.TotalEligible, TotalPages: totalPages}, nil
}

func (s *Service) ListOrders(ctx context.Context, status domain.ClaimStatus) ([]domain.MealOrder, error) {
	if status == "" {
		status = domain.ClaimRequested
	}
	if status != domain.ClaimRequested && status != domain.ClaimValidated {
		return nil, fmt.Errorf("status must be REQUESTED or VALIDATED")
	}
	return s.repo.ListOrders(ctx, status)
}

func (s *Service) FindOrder(ctx context.Context, id string) (*domain.MealOrder, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("order id is required")
	}
	return s.repo.FindOrderByID(ctx, id)
}

func (s *Service) ValidateOrder(ctx context.Context, id, collaboratorID string) (*domain.MealOrder, error) {
	order, err := s.repo.ValidateOrder(ctx, id, collaboratorID, s.now().In(s.peruLocation()))
	if err == nil {
		return order, nil
	}
	if !errors.Is(err, core.ErrNotFound) {
		return nil, err
	}
	existing, findErr := s.repo.FindOrderByID(ctx, id)
	if findErr != nil {
		return nil, findErr
	}
	if existing.Status == domain.ClaimValidated {
		return existing, fmt.Errorf("%w: order was already validated", core.ErrConflict)
	}
	if existing.Status == domain.ClaimNotConsumed || existing.Status == domain.ClaimRequestedNotValidated {
		return existing, fmt.Errorf("%w: finalized meal records cannot be validated", core.ErrConflict)
	}
	if existing.Status == domain.ClaimRequested {
		return existing, fmt.Errorf("%w: meal validation window is closed", core.ErrConflict)
	}
	return existing, core.ErrNotFound
}

func (s *Service) peruLocation() *time.Location {
	location, err := time.LoadLocation("America/Lima")
	if err != nil {
		return time.FixedZone("America/Lima", -5*60*60)
	}
	return location
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
	location := s.peruLocation()
	now := s.now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	status := &domain.WorkerStatus{PeruTime: now, AssignedMeals: make([]domain.AssignedMeal, 0)}
	rules, err := s.repo.ListRules(ctx)
	if err != nil {
		return nil, err
	}

	// El turno se determina con los horarios de comida configurados:
	// DIA: inicio de DESAYUNO -> fin de TARDE.
	// NOCHE: inicio de NOCHE -> fin de DESAYUNO del día siguiente.
	// Durante DESAYUNO ambos tipos pueden ser elegibles; se intenta primero
	// el turno NOCHE del día anterior y luego el turno DIA de hoy.
	var breakfastStart, breakfastEnd, afternoonEnd, nightStart *time.Time
	for _, rule := range rules {
		if !rule.Active {
			continue
		}
		start, parseErr := parseRuleTime(today, rule.ClaimStart)
		if parseErr != nil {
			return nil, parseErr
		}
		end, parseErr := parseRuleTime(today, rule.ClaimEnd)
		if parseErr != nil {
			return nil, parseErr
		}
		switch rule.MealType {
		case domain.Breakfast:
			breakfastStart, breakfastEnd = &start, &end
		case domain.Afternoon:
			afternoonEnd = &end
		case domain.Night:
			nightStart = &start
		}
	}

	type shiftCandidate struct {
		shiftType string
		workDate  time.Time
	}
	candidates := make([]shiftCandidate, 0, 2)
	if nightStart != nil && breakfastEnd != nil &&
		(!now.Before(*nightStart) || now.Before(*breakfastEnd)) {
		workDate := today
		if now.Before(*breakfastEnd) {
			workDate = today.AddDate(0, 0, -1)
		}
		candidates = append(candidates, shiftCandidate{"NOCHE", workDate})
	}
	if breakfastStart != nil && afternoonEnd != nil &&
		!now.Before(*breakfastStart) && now.Before(*afternoonEnd) {
		candidates = append(candidates, shiftCandidate{"DIA", today})
	}
	for _, candidate := range candidates {
		shift, findErr := s.repo.FindCurrentShift(ctx, workerID, candidate.shiftType, candidate.workDate)
		if findErr == nil {
			status.OnShift = true
			status.CurrentShift = shift
			break
		}
		if !errors.Is(findErr, core.ErrNotFound) {
			return nil, findErr
		}
	}

	if status.CurrentShift != nil {
		activeMeals := make(map[domain.MealType]bool, len(rules))
		for _, rule := range rules {
			if rule.Active {
				activeMeals[rule.MealType] = true
			}
		}
		appendMeal := func(mealType domain.MealType, displayName string, serviceDate time.Time) {
			if !activeMeals[mealType] {
				return
			}
			status.AssignedMeals = append(status.AssignedMeals, domain.AssignedMeal{
				MealType:    mealType,
				DisplayName: displayName,
				ServiceDate: serviceDate.Format("2006-01-02"),
			})
		}
		switch status.CurrentShift.ShiftType {
		case "DIA":
			appendMeal(domain.Breakfast, "Desayuno", status.CurrentShift.WorkDate)
			appendMeal(domain.Afternoon, "Almuerzo", status.CurrentShift.WorkDate)
		case "NOCHE":
			appendMeal(domain.Night, "Cena", status.CurrentShift.WorkDate)
			appendMeal(domain.Breakfast, "Desayuno", status.CurrentShift.WorkDate.AddDate(0, 0, 1))
		}
	}

	for _, rule := range rules {
		if !rule.Active {
			continue
		}
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
