package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	core "backend/internal/core/domain"
	users "backend/internal/modules/users/application"
	userdomain "backend/internal/modules/users/domain"
	"backend/internal/modules/workforce/domain"
)

type Service struct {
	repo  Repository
	users users.UserRepository
	now   func() time.Time
}

func NewService(repo Repository, users users.UserRepository, clocks ...func() time.Time) *Service {
	clock := time.Now
	if len(clocks) > 0 && clocks[0] != nil {
		clock = clocks[0]
	}
	return &Service{repo: repo, users: users, now: clock}
}

type RegisterWorkerInput struct {
	DNI, Email, FirstName, LastName, EmployeeCode      string
	JobTitle, Department, Phone, Address               string
	HireDate                                           *time.Time
	EmergencyContactName, EmergencyContactPhone, Notes string
}

func nullable(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func (s *Service) RegisterWorker(ctx context.Context, in RegisterWorkerInput) (*userdomain.User, *domain.WorkerInformation, error) {
	dni := nullable(in.DNI)
	if dni == nil || strings.TrimSpace(in.FirstName) == "" || strings.TrimSpace(in.LastName) == "" || strings.TrimSpace(in.EmployeeCode) == "" {
		return nil, nil, fmt.Errorf("dni, first_name, last_name and employee_code are required")
	}
	user := &userdomain.User{Role: userdomain.RoleWorker}
	info := &domain.WorkerInformation{FirstName: strings.TrimSpace(in.FirstName), LastName: strings.TrimSpace(in.LastName), Email: nullable(in.Email), EmployeeCode: strings.TrimSpace(in.EmployeeCode), JobTitle: nullable(in.JobTitle), Department: nullable(in.Department), Phone: nullable(in.Phone), Address: nullable(in.Address), HireDate: in.HireDate, EmergencyContactName: nullable(in.EmergencyContactName), EmergencyContactPhone: nullable(in.EmergencyContactPhone), Notes: nullable(in.Notes)}
	if err := s.repo.CreateWorker(ctx, user, info, *dni); err != nil {
		return nil, nil, err
	}
	return user, info, nil
}

func (s *Service) AssignWorker(ctx context.Context, workerID string, shiftType domain.ShiftType, assignedBy string, date time.Time, notes string) (*domain.WorkerShiftAssignment, error) {
	if date.IsZero() {
		return nil, fmt.Errorf("work_date is required")
	}
	date = s.peruDate(date)
	if !CanManageAssignmentForDate(date, s.peruToday()) {
		return nil, ErrAssignmentOutsideAllowedWeek
	}
	worker, err := s.users.FindByID(ctx, workerID)
	if err != nil || worker.Role != userdomain.RoleWorker || !worker.Active {
		return nil, fmt.Errorf("active WORKER not found")
	}
	if _, err = s.repo.FindWorkerInformation(ctx, workerID); err != nil {
		return nil, fmt.Errorf("worker information must be registered first")
	}
	if shiftType != domain.ShiftDay && shiftType != domain.ShiftNight {
		return nil, fmt.Errorf("shift_type must be DIA or NOCHE")
	}
	existingAssignment, err := s.repo.FindAssignmentByWorkerAndDate(ctx, workerID, date)
	if err == nil {
		return nil, &domain.AssignmentConflictError{Existing: *existingAssignment}
	}
	if !errors.Is(err, core.ErrNotFound) {
		return nil, err
	}
	a := &domain.WorkerShiftAssignment{WorkerID: workerID, ShiftType: shiftType, WorkDate: date, AssignedBy: assignedBy, Notes: nullable(notes)}
	if err = s.repo.CreateAssignment(ctx, a); err != nil {
		if errors.Is(err, core.ErrConflict) {
			if occupied, findErr := s.repo.FindAssignmentByWorkerAndDate(ctx, workerID, date); findErr == nil {
				return nil, &domain.AssignmentConflictError{Existing: *occupied}
			}
		}
		return nil, err
	}
	return a, nil
}

func (s *Service) UpdateAssignment(ctx context.Context, id string, shiftType domain.ShiftType, date time.Time, notes string) (*domain.WorkerShiftAssignment, error) {
	if shiftType != domain.ShiftDay && shiftType != domain.ShiftNight {
		return nil, fmt.Errorf("shift_type must be DIA or NOCHE")
	}
	today := s.peruToday()
	date = s.peruDate(date)

	existing, err := s.repo.FindAssignmentByID(ctx, id)

	if err != nil {
		return nil, err
	}

	existingDate := s.peruDate(existing.WorkDate)

	if !CanManageAssignmentForDate(existingDate, today) {
		return nil, ErrAssignmentOutsideAllowedWeek
	}

	if !CanManageAssignmentForDate(date, today) {
		return nil, ErrAssignmentOutsideAllowedWeek
	}
	if occupied, findErr := s.repo.FindAssignmentByWorkerAndDate(ctx, existing.WorkerID, date); findErr == nil && occupied.ID != existing.ID {
		return nil, &domain.AssignmentConflictError{Existing: *occupied}
	} else if findErr != nil && !errors.Is(findErr, core.ErrNotFound) {
		return nil, findErr
	}
	existing.ShiftType = shiftType
	existing.WorkDate = date
	existing.Notes = nullable(notes)
	if err = s.repo.UpdateAssignment(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *Service) DeleteAssignment(ctx context.Context, id string) error {
	today := s.peruToday()

	assignment, err := s.repo.FindAssignmentByID(ctx, id)
	if err != nil {
		return err
	}

	assignmentDate := s.peruDate(assignment.WorkDate)

	if !CanManageAssignmentForDate(assignmentDate, today) {
		return ErrAssignmentOutsideAllowedWeek
	}

	if err = s.repo.DeleteAssignment(ctx, id, today); err != nil {
		return err
	}

	return nil
}

func (s *Service) peruToday() time.Time { return s.peruDate(s.now().In(s.peruLocation())) }
func (s *Service) peruDate(value time.Time) time.Time {
	location := s.peruLocation()
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, location)
}
func (s *Service) peruLocation() *time.Location {
	location, err := time.LoadLocation("America/Lima")
	if err != nil {
		return time.FixedZone("America/Lima", -5*60*60)
	}
	return location
}

func (s *Service) ListAssignments(ctx context.Context, from, to time.Time) ([]domain.WorkerShiftAssignment, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, fmt.Errorf("valid from and to dates are required")
	}
	return s.repo.ListAssignments(ctx, from, to)
}

func (s *Service) ListWorkerAssignments(ctx context.Context, workerID, period string) ([]domain.WorkerShiftAssignment, error) {
	worker, err := s.users.FindByID(ctx, workerID)
	if err != nil || worker.Role != userdomain.RoleWorker {
		return nil, fmt.Errorf("WORKER not found")
	}
	if period != "past" && period != "upcoming" {
		return nil, fmt.Errorf("period must be past or upcoming")
	}
	return s.repo.ListWorkerAssignments(ctx, workerID, period, s.peruToday())
}

func (s *Service) ListWorkerAssignmentsRange(ctx context.Context, workerID string, from, to time.Time) ([]domain.WorkerShiftAssignment, error) {
	worker, err := s.users.FindByID(ctx, workerID)
	if err != nil || worker.Role != userdomain.RoleWorker {
		return nil, fmt.Errorf("WORKER not found")
	}
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, fmt.Errorf("valid from and to dates are required")
	}
	return s.repo.ListWorkerAssignmentsRange(ctx, workerID, s.peruDate(from), s.peruDate(to))
}

func (s *Service) ShiftPreview(
	ctx context.Context,
	date time.Time,
	mealTypes []string,
	page,
	pageSize int,
	paginate bool,
) (*domain.ShiftPreview, error) {

	// Fecha por defecto: hoy en Perú
	if date.IsZero() {
		date = s.peruToday()
	} else {
		date = s.peruDate(date)
	}

	// Filtro opcional por comidas
	mealFilter := make(map[string]bool)

	for _, value := range mealTypes {
		if value != "DESAYUNO" &&
			value != "TARDE" &&
			value != "NOCHE" {
			return nil, fmt.Errorf("invalid meal_type")
		}

		mealFilter[value] = true
	}

	// IMPORTANTE:
	// No filtramos por turno.
	// Traemos DIA y NOCHE.
	rows, err := s.repo.ListShiftPreview(ctx, date, nil)
	if err != nil {
		return nil, err
	}

	rules, err := s.repo.ListActiveMealRules(ctx)
	if err != nil {
		return nil, err
	}

	summary := domain.ShiftPreviewSummary{
		ByShift: map[string]int{
			"DIA":   0,
			"NOCHE": 0,
		},
		ByMeal: map[string]int{
			"DESAYUNO": 0,
			"TARDE":    0,
			"NOCHE":    0,
		},
	}

	filtered := make([]domain.ShiftPreviewRow, 0, len(rows))

	for _, row := range rows {

		// Evita duplicados si el objeto ya viniera con comidas
		row.AssignedMeals = nil

		for _, rule := range rules {

			eligible :=
				(row.ShiftType == domain.ShiftDay &&
					(rule.MealType == "DESAYUNO" ||
						rule.MealType == "TARDE")) ||
					(row.ShiftType == domain.ShiftNight &&
						(rule.MealType == "NOCHE" ||
							rule.MealType == "DESAYUNO"))

			if !eligible {
				continue
			}

			// Si mandaron filtro de comida, aplicarlo.
			// Si mealTypes está vacío, pasan todas.
			if len(mealFilter) > 0 && !mealFilter[rule.MealType] {
				continue
			}

			serviceDate := row.WorkDate

			// El desayuno de turno noche corresponde
			// al día siguiente.
			if row.ShiftType == domain.ShiftNight &&
				rule.MealType == "DESAYUNO" {
				serviceDate = serviceDate.AddDate(0, 0, 1)
			}

			displayName := map[string]string{
				"DESAYUNO": "DESAYUNO",
				"TARDE":    "ALMUERZO",
				"NOCHE":    "CENA",
			}[rule.MealType]

			row.AssignedMeals = append(
				row.AssignedMeals,
				domain.PreviewMeal{
					MealType:    rule.MealType,
					DisplayName: displayName,
					ServiceDate: serviceDate,
					Start:       strings.TrimSuffix(rule.Start, ":00"),
					End:         strings.TrimSuffix(rule.End, ":00"),
				},
			)
		}

		// Si se filtró por comida y este trabajador
		// no tiene esa comida, no aparece.
		if len(mealFilter) > 0 &&
			len(row.AssignedMeals) == 0 {
			continue
		}

		summary.TotalAssigned++

		// Seguimos devolviendo el turno del trabajador.
		summary.ByShift[string(row.ShiftType)]++

		for _, meal := range row.AssignedMeals {
			summary.ByMeal[meal.MealType]++
		}

		filtered = append(filtered, row)
	}

	// Defaults de paginación
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 20
	}

	if pageSize > 100 {
		pageSize = 100
	}

	total := len(filtered)

	totalPages := 0
	if total > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}

	data := filtered

	if paginate {
		start := (page - 1) * pageSize

		if start > total {
			start = total
		}

		end := start + pageSize

		if end > total {
			end = total
		}

		data = filtered[start:end]
	}

	return &domain.ShiftPreview{
		Date:       date,
		Summary:    summary,
		Data:       data,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}