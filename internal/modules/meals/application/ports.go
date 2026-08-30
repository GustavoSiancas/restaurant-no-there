package application

import (
	"context"
	"time"

	"backend/internal/modules/meals/domain"
)

type Repository interface {
	FindRule(ctx context.Context, mealType domain.MealType) (*domain.ServiceRule, error)
	FindEligibleAssignment(ctx context.Context, workerID string, mealType domain.MealType, serviceDate time.Time) (string, error)
	CreateClaim(ctx context.Context, claim *domain.Claim) error
	MarkConsumed(ctx context.Context, claimID, registeredBy string, consumedAt time.Time) (*domain.Claim, error)
	Report(ctx context.Context, from, to time.Time) ([]domain.ReportRow, error)
}
