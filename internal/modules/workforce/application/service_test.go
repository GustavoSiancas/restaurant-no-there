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

func (f fakeUsers) CreateManagement(context.Context, *userdomain.User, *userdomain.Profile, string, string) error {
	return nil
}
func (f fakeUsers) FindByID(context.Context, string) (*userdomain.User, error) { return f.user, nil }
func (f fakeUsers) FindMyUser(context.Context, string) (*userdomain.MyUser, error) {
	return nil, core.ErrNotFound
}
func (f fakeUsers) ListByRoles(context.Context, ...userdomain.Role) ([]userdomain.MyUser, error) {
	return nil, nil
}
func (f fakeUsers) FindPasswordCredential(context.Context, string) (*userdomain.User, string, error) {
	return nil, "", core.ErrNotFound
}
func (f fakeUsers) FindPasswordHashByUserID(context.Context, string) (string, error) {
	return "", core.ErrNotFound
}
func (f fakeUsers) UpdatePasswordHash(context.Context, string, string) error { return nil }
func (f fakeUsers) FindByDNI(context.Context, string) (*userdomain.User, error) {
	return nil, core.ErrNotFound
}
func (f fakeUsers) List(context.Context) ([]userdomain.User, error)           { return nil, nil }
func (f fakeUsers) RoleExists(context.Context, userdomain.Role) (bool, error) { return false, nil }

type fakeRepository struct {
	existing     *domain.WorkerShiftAssignment
	workerInfo   *domain.WorkerInformation
	created      bool
	bulkCreated  int
	bulkReplaced int
	preview      []domain.ShiftPreviewRow
	rules        []domain.PreviewRule
}

func (f *fakeRepository) CreateWorker(_ context.Context, _ *userdomain.User, info *domain.WorkerInformation, _ string) error {
	f.workerInfo = info
	return nil
}

func (f *fakeRepository) FindWorkerInformation(context.Context, string) (*domain.WorkerInformation, error) {
	return &domain.WorkerInformation{}, nil
}
func (f *fakeRepository) CreateAssignment(context.Context, *domain.WorkerShiftAssignment) error {
	f.created = true
	return nil
}
func (f *fakeRepository) ReplaceOpenAssignments(context.Context, []string, domain.ShiftType, time.Time, time.Time, string) (int, int, error) {
	f.created = true
	return f.bulkCreated, f.bulkReplaced, nil
}
func (f *fakeRepository) FindAssignmentByID(context.Context, string) (*domain.WorkerShiftAssignment, error) {
	if f.existing == nil {
		return nil, core.ErrNotFound
	}
	return f.existing, nil
}
func (f *fakeRepository) UpdateAssignment(context.Context, *domain.WorkerShiftAssignment) error {
	f.created = true
	return nil
}
func (f *fakeRepository) DeleteAssignment(context.Context, string, time.Time) error {
	f.created = true
	return nil
}
func (f *fakeRepository) FindAssignmentByWorkerAndDate(context.Context, string, time.Time) (*domain.WorkerShiftAssignment, error) {
	if f.existing != nil {
		return f.existing, nil
	}
	return nil, core.ErrNotFound
}
func (f *fakeRepository) ListWorkerAssignmentsRange(context.Context, string, time.Time, time.Time) ([]domain.WorkerShiftAssignment, error) {
	return nil, nil
}
func (f *fakeRepository) ListShiftPreview(context.Context, time.Time) ([]domain.ShiftPreviewRow, error) {
	return f.preview, nil
}
func (f *fakeRepository) ListActiveMealRules(context.Context) ([]domain.PreviewRule, error) {
	return f.rules, nil
}

func TestRegisterWorkerAcceptsOptionalPhotoURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *string
	}{
		{name: "omitted", input: "", want: nil},
		{name: "provided", input: "  https://cdn.example.com/worker.jpg  ", want: stringPointer("https://cdn.example.com/worker.jpg")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeRepository{}
			service := NewService(repo, fakeUsers{})
			_, info, err := service.RegisterWorker(context.Background(), RegisterWorkerInput{DNI: "12345678", FirstName: "Ana", LastName: "Pérez", EmployeeCode: "EMP-1", PhotoURL: test.input})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if (info.PhotoURL == nil) != (test.want == nil) || info.PhotoURL != nil && *info.PhotoURL != *test.want {
				t.Fatalf("unexpected photo URL: %#v", info.PhotoURL)
			}
			if repo.workerInfo != info {
				t.Fatal("worker info was not sent to repository")
			}
		})
	}
}

func stringPointer(value string) *string { return &value }

func TestShiftPreviewAssignsMealsByShift(t *testing.T) {
	date := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{preview: []domain.ShiftPreviewRow{{AssignmentID: "day", ShiftType: domain.ShiftDay, WorkDate: date, Worker: domain.PreviewWorker{ID: "day-worker"}}, {AssignmentID: "night", ShiftType: domain.ShiftNight, WorkDate: date, Worker: domain.PreviewWorker{ID: "night-worker"}}, {AssignmentID: "previous-night", ShiftType: domain.ShiftNight, WorkDate: date.AddDate(0, 0, -1), Worker: domain.PreviewWorker{ID: "previous-night-worker"}}}, rules: []domain.PreviewRule{{MealType: "BREAKFAST", Start: "06:00:00", End: "10:00:00"}, {MealType: "LUNCH", Start: "12:00:00", End: "15:00:00"}, {MealType: "DINNER", Start: "20:00:00", End: "23:00:00"}}}
	service := NewService(repo, fakeUsers{})
	report, err := service.ShiftPreview(context.Background(), date, nil, 1, 20, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Data) != 3 || report.Summary.ByMeal["BREAKFAST"] != 2 || report.Summary.ByMeal["LUNCH"] != 1 || report.Summary.ByMeal["DINNER"] != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	if len(report.Data[1].AssignedMeals) != 1 || report.Data[1].AssignedMeals[0].MealType != "DINNER" {
		t.Fatalf("current night must only include today's dinner: %+v", report.Data[1].AssignedMeals)
	}
	if len(report.Data[2].AssignedMeals) != 1 || report.Data[2].AssignedMeals[0].MealType != "BREAKFAST" || !report.Data[2].AssignedMeals[0].ServiceDate.Equal(date) {
		t.Fatalf("previous night must only include today's breakfast: %+v", report.Data[2].AssignedMeals)
	}
}

func TestAssignWorkerRejectsSecondShiftOnSameDate(t *testing.T) {
	repo := &fakeRepository{existing: &domain.WorkerShiftAssignment{ID: "existing"}}
	user := &userdomain.User{Entity: core.Entity{ID: "worker"}, Role: userdomain.RoleWorker, Active: true}
	service := NewService(repo, fakeUsers{user: user})
	service.now = func() time.Time { return time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC) }
	_, err := service.AssignWorker(context.Background(), "worker", domain.ShiftDay, "admin", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "")
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if repo.created {
		t.Fatal("a second assignment must not be created")
	}
}

func TestUpdateAssignmentIsLockedOnWorkDate(t *testing.T) {
	repo := &fakeRepository{existing: &domain.WorkerShiftAssignment{ID: "assignment", Status: domain.ShiftOpen, WorkDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}}
	service := NewService(repo, fakeUsers{})
	service.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.FixedZone("UTC-5", -5*60*60)) }
	_, err := service.UpdateAssignment(context.Background(), "assignment", domain.ShiftNight, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), "")
	if !errors.Is(err, core.ErrLocked) {
		t.Fatalf("expected locked error, got %v", err)
	}
	if repo.created {
		t.Fatal("locked assignment must not be updated")
	}
}

func TestDeleteAssignmentIsLockedOnWorkDate(t *testing.T) {
	repo := &fakeRepository{existing: &domain.WorkerShiftAssignment{ID: "assignment", Status: domain.ShiftOpen, WorkDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}}
	service := NewService(repo, fakeUsers{})
	service.now = func() time.Time { return time.Date(2026, 9, 1, 0, 1, 0, 0, time.FixedZone("UTC-5", -5*60*60)) }
	err := service.DeleteAssignment(context.Background(), "assignment")
	if !errors.Is(err, core.ErrLocked) {
		t.Fatalf("expected locked error, got %v", err)
	}
	if repo.created {
		t.Fatal("locked assignment must not be deleted")
	}
}

func TestUpdateAssignmentIsAllowedUntilPreviousDay2359InLima(t *testing.T) {
	peru := time.FixedZone("UTC-5", -5*60*60)
	repo := &fakeRepository{existing: &domain.WorkerShiftAssignment{
		ID: "assignment", WorkerID: "worker", Status: domain.ShiftOpen,
		WorkDate: time.Date(2026, 9, 2, 0, 0, 0, 0, peru),
	}}
	service := NewService(repo, fakeUsers{})
	service.now = func() time.Time { return time.Date(2026, 9, 1, 23, 59, 59, 0, peru) }

	_, err := service.UpdateAssignment(context.Background(), "assignment", domain.ShiftNight, time.Date(2026, 9, 2, 0, 0, 0, 0, peru), "")
	if err != nil {
		t.Fatalf("expected update to be allowed, got %v", err)
	}
	if !repo.created {
		t.Fatal("assignment should be updated before midnight")
	}
}

func TestAssignWorkerRequiresAtLeastTomorrow(t *testing.T) {
	peru := time.FixedZone("UTC-5", -5*60*60)
	tests := []struct {
		name      string
		date      time.Time
		wantError bool
	}{{name: "yesterday is rejected", date: time.Date(2026, 8, 29, 0, 0, 0, 0, peru), wantError: true}, {name: "today is rejected", date: time.Date(2026, 8, 30, 0, 0, 0, 0, peru), wantError: true}, {name: "tomorrow is allowed", date: time.Date(2026, 8, 31, 0, 0, 0, 0, peru)}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeRepository{}
			user := &userdomain.User{Entity: core.Entity{ID: "worker"}, Role: userdomain.RoleWorker, Active: true}
			service := NewService(repo, fakeUsers{user: user})
			service.now = func() time.Time { return time.Date(2026, 8, 30, 23, 59, 0, 0, peru) }
			_, err := service.AssignWorker(context.Background(), "worker", domain.ShiftDay, "rrhh", test.date, "")
			if test.wantError && err == nil {
				t.Fatal("expected date validation error")
			}
			if !test.wantError && err != nil {
				t.Fatalf("expected assignment to be allowed, got %v", err)
			}
			if repo.created == test.wantError {
				t.Fatalf("created=%v, want %v", repo.created, !test.wantError)
			}
		})
	}
}

func TestAddMassiveShiftWorkersAllowsTomorrowAndReturnsCounts(t *testing.T) {
	peru := time.FixedZone("UTC-5", -5*60*60)
	repo := &fakeRepository{bulkCreated: 14, bulkReplaced: 3}
	user := &userdomain.User{Entity: core.Entity{ID: "worker"}, Role: userdomain.RoleWorker, Active: true}
	service := NewService(repo, fakeUsers{user: user})
	service.now = func() time.Time { return time.Date(2026, 9, 1, 23, 59, 59, 0, peru) }

	result, err := service.AddMassiveShiftWorkers(context.Background(), []string{"worker", "worker"}, domain.ShiftDay, "rrhh", time.Date(2026, 9, 2, 0, 0, 0, 0, peru), time.Date(2026, 9, 15, 0, 0, 0, 0, peru))
	if err != nil {
		t.Fatalf("expected massive assignment to be allowed, got %v", err)
	}
	if !repo.created || result.Created != 14 || result.Replaced != 3 {
		t.Fatalf("unexpected massive assignment result: %+v", result)
	}
}

func TestAddMassiveShiftWorkersRejectsToday(t *testing.T) {
	peru := time.FixedZone("UTC-5", -5*60*60)
	repo := &fakeRepository{}
	user := &userdomain.User{Entity: core.Entity{ID: "worker"}, Role: userdomain.RoleWorker, Active: true}
	service := NewService(repo, fakeUsers{user: user})
	service.now = func() time.Time { return time.Date(2026, 9, 1, 10, 0, 0, 0, peru) }

	_, err := service.AddMassiveShiftWorkers(context.Background(), []string{"worker"}, domain.ShiftDay, "rrhh", time.Date(2026, 9, 1, 0, 0, 0, 0, peru), time.Date(2026, 9, 2, 0, 0, 0, 0, peru))
	if err == nil {
		t.Fatal("expected today's date to be rejected")
	}
	if repo.created {
		t.Fatal("massive assignments must not be created for today")
	}
}
