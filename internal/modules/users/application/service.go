package application

import (
	core "backend/internal/core/domain"
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
	if in.Role == domain.RoleWorker {
		return nil, fmt.Errorf("WORKER registration belongs to workforce service")
	}
	if strings.TrimSpace(in.Username) == "" || len(in.Password) < 8 || strings.TrimSpace(in.DNI) != "" {
		return nil, fmt.Errorf("management users require username and password (minimum 8 characters), without dni")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &domain.User{Role: in.Role}
	profile := &domain.Profile{FirstName: strings.TrimSpace(in.FirstName), LastName: strings.TrimSpace(in.LastName), Email: optional(in.Email)}
	if err := s.repo.CreateManagement(ctx, u, profile, strings.ToLower(strings.TrimSpace(in.Username)), string(hash)); err != nil {
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
func (s *Service) FindMyUser(ctx context.Context, id string) (*domain.MyUser, error) {
	return s.repo.FindMyUser(ctx, id)
}

func (s *Service) ListByRoles(ctx context.Context, roles ...domain.Role) ([]domain.MyUser, error) {
	return s.repo.ListByRoles(ctx, roles...)
}
func (s *Service) ListUsers(ctx context.Context) ([]domain.UserListItem, error) {
	return s.repo.ListUsers(ctx)
}

func validateNewPassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("new_password must contain at least 8 characters: %w", core.ErrInvalidInput)
	}
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	if strings.TrimSpace(userID) == "" || oldPassword == "" {
		return core.ErrUnauthorized
	}
	if err := validateNewPassword(newPassword); err != nil {
		return err
	}
	currentHash, err := s.repo.FindPasswordHashByUserID(ctx, userID)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(oldPassword)) != nil {
		return core.ErrUnauthorized
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePasswordHash(ctx, userID, string(newHash))
}

func (s *Service) ResetPassword(ctx context.Context, userID, newPassword string) error {
	if strings.TrimSpace(userID) == "" {
		return core.ErrNotFound
	}
	if err := validateNewPassword(newPassword); err != nil {
		return err
	}
	_, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePasswordHash(ctx, userID, string(newHash))
}
