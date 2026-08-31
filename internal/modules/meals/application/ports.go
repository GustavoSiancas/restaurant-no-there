package application

import (
	"context"
	"time"

	"backend/internal/modules/meals/domain"
)

type Repository interface {
	FindRule(ctx context.Context, mealType domain.MealType) (*domain.ServiceRule, error)
	ListRules(ctx context.Context) ([]domain.ServiceRule, error)
	FindEligibleAssignment(ctx context.Context, workerID string, mealType domain.MealType, serviceDate time.Time) (string, error)
	FindCurrentShift(ctx context.Context, workerID, shiftType string, workDate time.Time) (*domain.CurrentShift, error)
	FindClaim(ctx context.Context, workerID string, mealType domain.MealType, serviceDate time.Time) (*domain.Claim, error)
	FindWorkerTicketIdentity(ctx context.Context, workerID string) (*domain.WorkerTicketIdentity, error)
	CreateClaim(ctx context.Context, claim *domain.Claim) error
	Report(ctx context.Context, from, to time.Time) ([]domain.ReportRow, error)
}
