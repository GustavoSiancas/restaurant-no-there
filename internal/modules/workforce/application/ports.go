package application

import (
	"context"
	"time"

	"backend/internal/modules/workforce/domain"
)

type Repository interface {
	CreateWorkerInformation(ctx context.Context, info *domain.WorkerInformation) error
	FindWorkerInformation(ctx context.Context, userID string) (*domain.WorkerInformation, error)
	CreateShift(ctx context.Context, shift *domain.Shift) error
	FindShift(ctx context.Context, id string) (*domain.Shift, error)
	ListShifts(ctx context.Context) ([]domain.Shift, error)
	CreateAssignment(ctx context.Context, assignment *domain.WorkerShiftAssignment) error
	FindAssignmentByWorkerAndDate(ctx context.Context, workerID string, date time.Time) (*domain.WorkerShiftAssignment, error)
	ListAssignments(ctx context.Context, from, to time.Time) ([]domain.WorkerShiftAssignment, error)
}
