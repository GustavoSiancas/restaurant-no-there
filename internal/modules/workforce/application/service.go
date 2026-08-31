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

	// ============================================================
	// 1. NORMALIZAR FECHA
	// Si no llega fecha, usar hoy en Perú.
	// ============================================================
	if date.IsZero() {
		date = s.peruToday()
	} else {
		date = s.peruDate(date)
	}

	fmt.Printf(
		"[SHIFT_PREVIEW] start date=%s mealTypes=%v page=%d pageSize=%d paginate=%v\n",
		date.Format("2006-01-02"),
		mealTypes,
		page,
		pageSize,
		paginate,
	)

	// ============================================================
	// 2. VALIDAR FILTRO DE COMIDAS
	// Si mealTypes está vacío => no se filtra => muestra todas.
	// ============================================================
	mealFilter := make(map[string]bool)

	for _, value := range mealTypes {
		if value != "DESAYUNO" &&
			value != "TARDE" &&
			value != "NOCHE" {

			fmt.Printf(
				"[SHIFT_PREVIEW] invalid meal_type=%s\n",
				value,
			)

			return nil, fmt.Errorf("invalid meal_type")
		}

		mealFilter[value] = true
	}

	fmt.Printf(
		"[SHIFT_PREVIEW] mealFilter=%v count=%d\n",
		mealFilter,
		len(mealFilter),
	)

	// ============================================================
	// 3. TRAER LOS TURNOS DE LA FECHA
	// Queremos DIA + NOCHE.
	//
	// Si acá rows=0, el problema está en el repository.
	// ============================================================
	rows, err := s.repo.ListShiftPreview(ctx, date)
	if err != nil {
		fmt.Printf(
			"[SHIFT_PREVIEW] ERROR ListShiftPreview date=%s err=%v\n",
			date.Format("2006-01-02"),
			err,
		)

		return nil, err
	}

	fmt.Printf(
		"[SHIFT_PREVIEW] repository rows=%d date=%s\n",
		len(rows),
		date.Format("2006-01-02"),
	)

	for i, row := range rows {
		fmt.Printf(
			"[SHIFT_PREVIEW] row[%d] assignmentID=%s shift=%s workDate=%s\n",
			i,
			row.AssignmentID,
			row.ShiftType,
			row.WorkDate.Format("2006-01-02"),
		)
	}

	// ============================================================
	// 4. TRAER REGLAS ACTIVAS DE COMIDA
	// ============================================================
	rules, err := s.repo.ListActiveMealRules(ctx)
	if err != nil {
		fmt.Printf(
			"[SHIFT_PREVIEW] ERROR ListActiveMealRules err=%v\n",
			err,
		)

		return nil, err
	}

	fmt.Printf(
		"[SHIFT_PREVIEW] active meal rules=%d\n",
		len(rules),
	)

	for i, rule := range rules {
		fmt.Printf(
			"[SHIFT_PREVIEW] rule[%d] meal=%s start=%s end=%s\n",
			i,
			rule.MealType,
			rule.Start,
			rule.End,
		)
	}

	// ============================================================
	// 5. INICIALIZAR RESUMEN
	// ============================================================
	summary := domain.ShiftPreviewSummary{
		ByMeal: map[string]int{
			"DESAYUNO": 0,
			"TARDE":    0,
			"NOCHE":    0,
		},
	}

	filtered := make(
		[]domain.ShiftPreviewRow,
		0,
		len(rows),
	)
	seenMeals := make(map[string]bool)

	// ============================================================
	// 6. PROCESAR CADA TURNO
	// ============================================================
	for _, row := range rows {

		// Limpiar por seguridad si viniera ya con comidas.
		row.AssignedMeals = nil

		fmt.Printf(
			"[SHIFT_PREVIEW] processing assignmentID=%s shift=%s\n",
			row.AssignmentID,
			row.ShiftType,
		)

		for _, rule := range rules {

			// ====================================================
			// REGLA DE COMIDAS SEGÚN TURNO
			//
			// DIA:
			// - DESAYUNO
			// - TARDE / ALMUERZO
			//
			// NOCHE:
			// - NOCHE / CENA
			// - DESAYUNO del día siguiente
			// ====================================================
			eligible := false

			if row.ShiftType == domain.ShiftDay {
				eligible =
					rule.MealType == "DESAYUNO" ||
						rule.MealType == "TARDE"
			}

			if row.ShiftType == domain.ShiftNight {
				eligible =
					rule.MealType == "NOCHE" ||
						rule.MealType == "DESAYUNO"
			}

			fmt.Printf(
				"[SHIFT_PREVIEW] check assignmentID=%s shift=%s meal=%s eligible=%v\n",
				row.AssignmentID,
				row.ShiftType,
				rule.MealType,
				eligible,
			)

			if !eligible {
				continue
			}

			serviceDate := row.WorkDate

			// El desayuno de turno noche corresponde al día siguiente.
			if row.ShiftType == domain.ShiftNight &&
				rule.MealType == "DESAYUNO" {
				serviceDate = serviceDate.AddDate(0, 0, 1)
			}

			// Este endpoint representa las comidas que deben prepararse en
			// la fecha solicitada, no todas las comidas del turno consultado.
			if serviceDate.Format("2006-01-02") != date.Format("2006-01-02") {
				continue
			}

			mealKey := row.Worker.ID + "|" + rule.MealType + "|" + serviceDate.Format("2006-01-02")
			if seenMeals[mealKey] {
				continue
			}

			// ====================================================
			// FILTRO DE COMIDA
			// Si no mandaron meal_type, mealFilter está vacío
			// y todas las comidas pasan.
			// ====================================================
			if len(mealFilter) > 0 &&
				!mealFilter[rule.MealType] {

				fmt.Printf(
					"[SHIFT_PREVIEW] meal filtered assignmentID=%s meal=%s\n",
					row.AssignmentID,
					rule.MealType,
				)

				continue
			}

			displayName := ""

			switch rule.MealType {
			case "DESAYUNO":
				displayName = "DESAYUNO"
			case "TARDE":
				displayName = "ALMUERZO"
			case "NOCHE":
				displayName = "CENA"
			}

			row.AssignedMeals = append(
				row.AssignedMeals,
				domain.PreviewMeal{
					MealType:    rule.MealType,
					DisplayName: displayName,
					ServiceDate: serviceDate,
					Start: strings.TrimSuffix(
						rule.Start,
						":00",
					),
					End: strings.TrimSuffix(
						rule.End,
						":00",
					),
				},
			)
			seenMeals[mealKey] = true

			fmt.Printf(
				"[SHIFT_PREVIEW] meal added assignmentID=%s meal=%s serviceDate=%s\n",
				row.AssignmentID,
				rule.MealType,
				serviceDate.Format("2006-01-02"),
			)
		}

		// ========================================================
		// Si el turno no aporta ninguna comida para la fecha solicitada,
		// no forma parte de la lista de preparación.
		// ========================================================
		if len(row.AssignedMeals) == 0 {

			fmt.Printf(
				"[SHIFT_PREVIEW] worker excluded assignmentID=%s reason=no_matching_meal\n",
				row.AssignmentID,
			)

			continue
		}
		row.MealDate = row.AssignedMeals[0].ServiceDate.Format("2006-01-02")

		// ========================================================
		// CONTADORES
		// ========================================================
		for _, meal := range row.AssignedMeals {
			summary.ByMeal[meal.MealType]++
		}

		filtered = append(filtered, row)

		fmt.Printf(
			"[SHIFT_PREVIEW] worker included assignmentID=%s shift=%s meals=%d\n",
			row.AssignmentID,
			row.ShiftType,
			len(row.AssignedMeals),
		)
	}

	// ============================================================
	// 7. PAGINACIÓN
	// ============================================================
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
		totalPages =
			(total + pageSize - 1) / pageSize
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

	// ============================================================
	// 8. LOG FINAL
	// ============================================================
	fmt.Printf(
		"[SHIFT_PREVIEW] finished date=%s rowsRepo=%d filtered=%d pageData=%d totalPages=%d byMeal=%v\n",
		date.Format("2006-01-02"),
		len(rows),
		len(filtered),
		len(data),
		totalPages,
		summary.ByMeal,
	)

	return &domain.ShiftPreview{
		Date:       date,
		Summary:    summary,
		Data:       data,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}
