package application

import (
	"context"

	"backend/internal/modules/users/domain"
)

type UserRepository interface {
	CreateManagement(ctx context.Context, user *domain.User, profile *domain.Profile, username, passwordHash string) error
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindMyUser(ctx context.Context, id string) (*domain.MyUser, error)
	ListByRoles(ctx context.Context, roles ...domain.Role) ([]domain.MyUser, error)
	FindPasswordCredential(ctx context.Context, username string) (*domain.User, string, error)
	FindPasswordHashByUserID(ctx context.Context, userID string) (string, error)
	UpdatePasswordHash(ctx context.Context, userID, passwordHash string) error
	FindByDNI(ctx context.Context, dni string) (*domain.User, error)
	RoleExists(ctx context.Context, role domain.Role) (bool, error)
}
