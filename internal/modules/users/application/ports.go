package application

import (
	"context"

	"backend/internal/modules/users/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	FindByDNI(ctx context.Context, dni string) (*domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
}
