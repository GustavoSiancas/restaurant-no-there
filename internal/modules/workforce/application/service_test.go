package application

import (
	"context"
	"errors"
	"testing"
	"time"

	core "backend/internal/core/domain"
	userdomain "backend/internal/modules/users/domain"
	"backend/internal/modules/workforce/domain"
)

type fakeUsers struct{ user *userdomain.User }

func (f fakeUsers) Create(context.Context, *userdomain.User) error             { return nil }
func (f fakeUsers) FindByID(context.Context, string) (*userdomain.User, error) { return f.user, nil }
func (f fakeUsers) FindByUsername(context.Context, string) (*userdomain.User, error) {
	return nil, core.ErrNotFound
}
func (f fakeUsers) FindByDNI(context.Context, string) (*userdomain.User, error) {
	return nil, core.ErrNotFound
}
func (f fakeUsers) List(context.Context) ([]userdomain.User, error)           { return nil, nil }
func (f fakeUsers) RoleExists(context.Context, userdomain.Role) (bool, error) { return false, nil }

type fakeRepository struct {
	existing *domain.WorkerShiftAssignment
	created  bool
}

func (f *fakeRepository) CreateWorker(context.Context, *userdomain.User, *domain.WorkerInformation) error {
	return nil
}

func (f *fakeRepository) FindWorkerInformation(context.Context, string) (*domain.WorkerInformation, error) {
	return &domain.WorkerInformation{}, nil
}
func (f *fakeRepository) CreateShift(context.Context, *domain.Shift) error { return nil }
func (f *fakeRepository) FindShift(context.Context, string) (*domain.Shift, error) {
	return &domain.Shift{Active: true}, nil
}
func (f *fakeRepository) ListShifts(context.Context) ([]domain.Shift, error) { return nil, nil }
func (f *fakeRepository) CreateAssignment(context.Context, *domain.WorkerShiftAssignment) error {
	f.created = true
	return nil
}
func (f *fakeRepository) FindAssignmentByWorkerAndDate(context.Context, string, time.Time) (*domain.WorkerShiftAssignment, error) {
	if f.existing != nil {
		return f.existing, nil
	}
	return nil, core.ErrNotFound
}
func (f *fakeRepository) ListAssignments(context.Context, time.Time, time.Time) ([]domain.WorkerShiftAssignment, error) {
	return nil, nil
}

func TestAssignWorkerRejectsSecondShiftOnSameDate(t *testing.T) {
	repo := &fakeRepository{existing: &domain.WorkerShiftAssignment{ID: "existing"}}
	user := &userdomain.User{Entity: core.Entity{ID: "worker"}, Role: userdomain.RoleWorker, Active: true}
	service := NewService(repo, fakeUsers{user: user})
	_, err := service.AssignWorker(context.Background(), "worker", "shift", "admin", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "")
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if repo.created {
		t.Fatal("a second assignment must not be created")
	}
}
