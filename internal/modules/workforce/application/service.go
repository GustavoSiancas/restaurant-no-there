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
}

func NewService(repo Repository, users users.UserRepository) *Service {
	return &Service{repo: repo, users: users}
}

type RegisterWorkerInput struct {
	DNI, Email, FirstName, LastName, EmployeeCode      string
	JobTitle, Department, Phone, Address               string
	HireDate                                           *time.Time
	EmergencyContactName, EmergencyContactPhone, Notes string
}

type ShiftInput struct {
	Name, Description, StartTime, EndTime string
	Type                                  domain.ShiftType
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
	user := &userdomain.User{DNI: dni, Email: nullable(in.Email), FirstName: strings.TrimSpace(in.FirstName), LastName: strings.TrimSpace(in.LastName), Role: userdomain.RoleWorker}
	info := &domain.WorkerInformation{EmployeeCode: strings.TrimSpace(in.EmployeeCode), JobTitle: nullable(in.JobTitle), Department: nullable(in.Department), Phone: nullable(in.Phone), Address: nullable(in.Address), HireDate: in.HireDate, EmergencyContactName: nullable(in.EmergencyContactName), EmergencyContactPhone: nullable(in.EmergencyContactPhone), Notes: nullable(in.Notes)}
	if err := s.repo.CreateWorker(ctx, user, info); err != nil {
		return nil, nil, err
	}
	return user, info, nil
}

func (s *Service) CreateShift(ctx context.Context, in ShiftInput) (*domain.Shift, error) {
	if in.Type != domain.ShiftDay && in.Type != domain.ShiftNight {
		return nil, fmt.Errorf("type must be DIA or NOCHE")
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Description) == "" {
		return nil, fmt.Errorf("name and description are required")
	}
	start, err := time.Parse("15:04", in.StartTime)
	if err != nil {
		return nil, fmt.Errorf("start_time must use HH:MM")
	}
	end, err := time.Parse("15:04", in.EndTime)
	if err != nil {
		return nil, fmt.Errorf("end_time must use HH:MM")
	}
	if start.Equal(end) {
		return nil, fmt.Errorf("start_time and end_time must be different")
	}
	shift := &domain.Shift{Name: strings.TrimSpace(in.Name), Type: in.Type, Description: strings.TrimSpace(in.Description), StartTime: in.StartTime, EndTime: in.EndTime, Active: true}
	if err = s.repo.CreateShift(ctx, shift); err != nil {
		return nil, err
	}
	return shift, nil
}

func (s *Service) ListShifts(ctx context.Context) ([]domain.Shift, error) {
	return s.repo.ListShifts(ctx)
}

func (s *Service) AssignWorker(ctx context.Context, workerID, shiftID, assignedBy string, date time.Time, notes string) (*domain.WorkerShiftAssignment, error) {
	if date.IsZero() {
		return nil, fmt.Errorf("work_date is required")
	}
	worker, err := s.users.FindByID(ctx, workerID)
	if err != nil || worker.Role != userdomain.RoleWorker || !worker.Active {
		return nil, fmt.Errorf("active WORKER not found")
	}
	if _, err = s.repo.FindWorkerInformation(ctx, workerID); err != nil {
		return nil, fmt.Errorf("worker information must be registered first")
	}
	shift, err := s.repo.FindShift(ctx, shiftID)
	if err != nil || !shift.Active {
		return nil, fmt.Errorf("active shift not found")
	}
	_, err = s.repo.FindAssignmentByWorkerAndDate(ctx, workerID, date)
	if err == nil {
		return nil, fmt.Errorf("%w: worker already has a shift on this date", core.ErrConflict)
	}
	if !errors.Is(err, core.ErrNotFound) {
		return nil, err
	}
	a := &domain.WorkerShiftAssignment{WorkerID: workerID, ShiftID: shiftID, WorkDate: date, AssignedBy: assignedBy, Notes: nullable(notes)}
	if err = s.repo.CreateAssignment(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) ListAssignments(ctx context.Context, from, to time.Time) ([]domain.WorkerShiftAssignment, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, fmt.Errorf("valid from and to dates are required")
	}
	return s.repo.ListAssignments(ctx, from, to)
}
