package application

import (
	"backend/internal/modules/users/domain"
	"context"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"strings"
)

type RegisterInput struct {
	Username, DNI, Email, Password, FirstName, LastName string
	Role                                                domain.Role
}
type Service struct{ repo UserRepository }

func NewService(repo UserRepository) *Service { return &Service{repo: repo} }

func optional(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
func (s *Service) Register(ctx context.Context, in RegisterInput) (*domain.User, error) {
	if !in.Role.Valid() || strings.TrimSpace(in.FirstName) == "" || strings.TrimSpace(in.LastName) == "" {
		return nil, fmt.Errorf("role, first_name and last_name are required")
	}
	u := &domain.User{Username: optional(in.Username), DNI: optional(in.DNI), Email: optional(in.Email), FirstName: strings.TrimSpace(in.FirstName), LastName: strings.TrimSpace(in.LastName), Role: in.Role}
	if in.Role == domain.RoleWorker {
		if u.DNI == nil || u.Username != nil || in.Password != "" {
			return nil, fmt.Errorf("WORKER requires only dni and must not contain username or password")
		}
	} else {
		if u.Username == nil || len(in.Password) < 8 || u.DNI != nil {
			return nil, fmt.Errorf("ADMIN, OWNER and RRHH require username and password (minimum 8 characters), without dni")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		value := string(hash)
		u.PasswordHash = &value
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) BootstrapAdmin(ctx context.Context, in RegisterInput) (*domain.User, error) {
	exists, err := s.repo.RoleExists(ctx, domain.RoleAdmin)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("the initial ADMIN has already been created")
	}
	in.Role = domain.RoleAdmin
	return s.Register(ctx, in)
}
func (s *Service) List(ctx context.Context) ([]domain.User, error) { return s.repo.List(ctx) }
func (s *Service) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.FindByID(ctx, id)
}
