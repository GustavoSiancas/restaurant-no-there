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

func NewService(repo Repository, users users.UserRepository) *Service {
	return &Service{repo: repo, users: users, now: time.Now}
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
	if !date.After(s.peruToday()) {
		return nil, fmt.Errorf("work_date must be at least one day in advance")
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
	_, err = s.repo.FindAssignmentByWorkerAndDate(ctx, workerID, date)
	if err == nil {
		return nil, fmt.Errorf("%w: worker already has a shift on this date", core.ErrConflict)
	}
	if !errors.Is(err, core.ErrNotFound) {
		return nil, err
	}
	a := &domain.WorkerShiftAssignment{WorkerID: workerID, ShiftType: shiftType, WorkDate: date, AssignedBy: assignedBy, Notes: nullable(notes)}
	if err = s.repo.CreateAssignment(ctx, a); err != nil {
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
	if !s.peruDate(existing.WorkDate).After(today) {
		return nil, fmt.Errorf("%w: shifts cannot be modified on or after their work date", core.ErrLocked)
	}
	if !date.After(today) {
		return nil, fmt.Errorf("work_date must be at least one day in advance")
	}
	existing.ShiftType = shiftType
	existing.WorkDate = date
	existing.Notes = nullable(notes)
	if err = s.repo.UpdateAssignment(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
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
