package application

import (
	"context"
	"time"

	userdomain "backend/internal/modules/users/domain"
	"backend/internal/modules/workforce/domain"
)

type Repository interface {
	CreateWorker(ctx context.Context, user *userdomain.User, info *domain.WorkerInformation, dni string) error
	FindWorkerInformation(ctx context.Context, userID string) (*domain.WorkerInformation, error)
	CreateAssignment(ctx context.Context, assignment *domain.WorkerShiftAssignment) error
	FindAssignmentByID(ctx context.Context, id string) (*domain.WorkerShiftAssignment, error)
	FindAssignmentByWorkerAndDate(ctx context.Context, workerID string, date time.Time) (*domain.WorkerShiftAssignment, error)
	UpdateAssignment(ctx context.Context, assignment *domain.WorkerShiftAssignment) error
	DeleteAssignment(ctx context.Context, id string, today time.Time) error
	ListAssignments(ctx context.Context, from, to time.Time) ([]domain.WorkerShiftAssignment, error)
	ListWorkerAssignments(ctx context.Context, workerID, period string, today time.Time) ([]domain.WorkerShiftAssignment, error)
	ListWorkerAssignmentsRange(ctx context.Context, workerID string, from, to time.Time) ([]domain.WorkerShiftAssignment, error)
}
