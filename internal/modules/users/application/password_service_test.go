package application

import (
	"context"
	"testing"

	core "backend/internal/core/domain"
	"backend/internal/modules/users/domain"
	"golang.org/x/crypto/bcrypt"
)

type passwordRepository struct {
	user        *domain.User
	currentHash string
	updatedHash string
}

func (r *passwordRepository) CreateManagement(context.Context, *domain.User, *domain.Profile, string, string) error {
	return nil
}
func (r *passwordRepository) FindByID(context.Context, string) (*domain.User, error) {
	if r.user == nil {
		return nil, core.ErrNotFound
	}
	return r.user, nil
}
func (r *passwordRepository) FindMyUser(context.Context, string) (*domain.MyUser, error) {
	return nil, core.ErrNotFound
}
func (r *passwordRepository) ListByRoles(context.Context, ...domain.Role) ([]domain.MyUser, error) {
	return nil, nil
}
func (r *passwordRepository) FindPasswordCredential(context.Context, string) (*domain.User, string, error) {
	return nil, "", core.ErrNotFound
}
func (r *passwordRepository) FindPasswordHashByUserID(context.Context, string) (string, error) {
	if r.currentHash == "" {
		return "", core.ErrNotFound
	}
	return r.currentHash, nil
}
func (r *passwordRepository) UpdatePasswordHash(_ context.Context, _ string, hash string) error {
	r.updatedHash = hash
	return nil
}
func (r *passwordRepository) FindByDNI(context.Context, string) (*domain.User, error) {
	return nil, core.ErrNotFound
}
func (r *passwordRepository) RoleExists(context.Context, domain.Role) (bool, error) {
	return false, nil
}

func TestChangePasswordVerifiesOldPasswordAndHashesNewPassword(t *testing.T) {
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repository := &passwordRepository{currentHash: string(oldHash)}
	service := NewService(repository)

	if err = service.ChangePassword(context.Background(), "user-1", "old-password", "new-password"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(repository.updatedHash), []byte("new-password")) != nil {
		t.Fatal("new password was not stored as a bcrypt hash")
	}
}

func TestChangePasswordRejectsIncorrectOldPassword(t *testing.T) {
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repository := &passwordRepository{currentHash: string(oldHash)}
	err = NewService(repository).ChangePassword(context.Background(), "user-1", "incorrect", "new-password")
	if err != core.ErrUnauthorized || repository.updatedHash != "" {
		t.Fatalf("expected unauthorized without update, got %v", err)
	}
}

func TestResetPasswordRejectsWorker(t *testing.T) {
	repository := &passwordRepository{user: &domain.User{Role: domain.RoleWorker}}
	err := NewService(repository).ResetPassword(context.Background(), "worker-1", "new-password")
	if err == nil || repository.updatedHash != "" {
		t.Fatalf("expected WORKER reset to be rejected, got %v", err)
	}
}

func TestResetPasswordHashesManagementUserPassword(t *testing.T) {
	repository := &passwordRepository{user: &domain.User{Role: domain.RoleCollaborator}}
	if err := NewService(repository).ResetPassword(context.Background(), "user-1", "new-password"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(repository.updatedHash), []byte("new-password")) != nil {
		t.Fatal("reset password was not stored as a bcrypt hash")
	}
}
