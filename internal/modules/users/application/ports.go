package application

import (
	"context"

	"backend/internal/modules/users/domain"
)

type UserRepository interface {
	CreateManagement(ctx context.Context, user *domain.User, profile *domain.Profile, username, passwordHash string) error
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindPasswordCredential(ctx context.Context, username string) (*domain.User, string, error)
	FindByDNI(ctx context.Context, dni string) (*domain.User, error)
	List(ctx context.Context) ([]domain.User, error)
	RoleExists(ctx context.Context, role domain.Role) (bool, error)
}
