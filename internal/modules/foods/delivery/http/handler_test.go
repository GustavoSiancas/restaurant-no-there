package http

import (
	core "backend/internal/core/domain"
	"backend/internal/modules/foods/application"
	"backend/internal/modules/foods/domain"
	"context"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeRepository struct {
	update      *application.UpdateFoodInput
	requestedID string
	ids         []string
	food        *domain.Food
	page, size  int
	err         error
}

func (r *fakeRepository) GetFood(_ context.Context, id string) (*domain.Food, error) {
	r.requestedID = id
	return &domain.Food{Tags: []domain.Tag{}}, r.err
}
func (r *fakeRepository) UpdateFood(_ context.Context, id string, in application.UpdateFoodInput) (*domain.Food, error) {
	r.requestedID = id
	r.update = &in
	return &domain.Food{Tags: []domain.Tag{}}, r.err
}

func TestGetAndUpdateFood(t *testing.T) {
	const id = "00000000-0000-0000-0000-000000000001"
	for _, tc := range []struct {
		name, method, id, body string
		err                    error
		status                 int
	}{
		{"get", "GET", id, "", nil, 200},
		{"missing", "GET", id, "", core.ErrNotFound, 404},
		{"invalid id", "GET", "bad", "", nil, 400},
		{"replace tags", "PUT", id, `{"tag_ids":["` + id + `","` + id + `"]}`, nil, 200},
		{"clear tags", "PUT", id, `{"tag_ids":[]}`, nil, 200},
		{"keep tags", "PUT", id, `{"name":" Rice ","total_calories":0}`, nil, 200},
		{"bad tags", "PUT", id, `{"tag_ids":["bad"]}`, nil, 400},
		{"bad calories", "PUT", id, `{"total_calories":-1}`, nil, 400},
		{"blank name", "PUT", id, `{"name":" "}`, nil, 400},
		{"missing update", "PUT", id, `{"tag_ids":[]}`, core.ErrNotFound, 404},
		{"unknown tag", "PUT", id, `{"tag_ids":["` + id + `"]}`, core.ErrInvalidInput, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{err: tc.err}
			w := httptest.NewRecorder()
			router(repo).ServeHTTP(w, httptest.NewRequest(tc.method, "/foods/"+tc.id, strings.NewReader(tc.body)))
			if w.Code != tc.status {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if tc.name == "replace tags" && len(repo.update.TagIDs) != 1 {
				t.Fatal("tags not deduplicated")
			}
			if tc.name == "clear tags" && (repo.update.TagIDs == nil || len(repo.update.TagIDs) != 0) {
				t.Fatal("empty tags must replace all tags")
			}
			if tc.name == "keep tags" && (repo.update.TagIDs != nil || *repo.update.Name != "Rice" || *repo.update.TotalCalories != 0) {
				t.Fatal("omitted tags or zero calories mishandled")
			}
		})
	}
}

func (r *fakeRepository) CreateFood(_ context.Context, f *domain.Food, ids []string) error {
	r.food = f
	r.ids = ids
	return r.err
}
func (r *fakeRepository) ListFoods(_ context.Context, page, size int, ids []string) (*application.FoodPage, error) {
	r.page = page
	r.size = size
	r.ids = ids
	return &application.FoodPage{Data: []domain.Food{}, Page: page, PageSize: size}, r.err
}
func (r *fakeRepository) CreateTag(_ context.Context, t *domain.Tag) error { return r.err }
func (r *fakeRepository) ListTags(context.Context) ([]domain.Tag, error) {
	return []domain.Tag{}, r.err
}
func router(r *fakeRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	h := New(application.NewService(r))
	e.POST("/foods", h.CreateFood)
	e.GET("/foods", h.ListFoods)
	e.GET("/foods/:id", h.GetFood)
	e.PUT("/foods/:id", h.UpdateFood)
	e.POST("/tags", h.CreateTag)
	e.GET("/tags", h.ListTags)
	return e
}
func TestCreateFoodValidation(t *testing.T) {
	for _, tc := range []struct {
		name, calories, tags string
		want                 int
	}{
		{"decimal", "123.5", `[]`, 201}, {"zero", "0", `[]`, 201}, {"negative", "-1", `[]`, 400}, {"null", "null", `[]`, 400}, {"string", `"123"`, `[]`, 400}, {"bad tag", "1", `["invalid"]`, 400},
		{"duplicate tags", "1", `["00000000-0000-0000-0000-000000000001","00000000-0000-0000-0000-000000000001"]`, 201},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{}
			w := httptest.NewRecorder()
			body := `{"name":" Rice ","long_description":"Rice dish","photo_url":"https://example.com/rice.jpg","total_calories":` + tc.calories + `,"tag_ids":` + tc.tags + `}`
			router(repo).ServeHTTP(w, httptest.NewRequest("POST", "/foods", strings.NewReader(body)))
			if w.Code != tc.want {
				t.Fatalf("status %d: %s", w.Code, w.Body.String())
			}
			if tc.want == 400 && repo.food != nil {
				t.Fatal("invalid request reached repository")
			}
			if tc.name == "decimal" && repo.food.TotalCalories != 123.5 {
				t.Fatal("decimal lost")
			}
			if tc.name == "duplicate tags" && len(repo.ids) != 1 {
				t.Fatal("tags not deduplicated")
			}
		})
	}
}
func TestListFoods(t *testing.T) {
	for _, tc := range []struct {
		query                 string
		want, page, size, ids int
	}{
		{"", 200, 1, 20, 0}, {"?page=2&page_size=5&tag_ids=00000000-0000-0000-0000-000000000001,00000000-0000-0000-0000-000000000002", 200, 2, 5, 2},
		{"?page=0", 400, 0, 0, 0}, {"?page_size=101", 400, 0, 0, 0}, {"?page=x", 400, 0, 0, 0}, {"?tag_ids=bad", 400, 0, 0, 0},
	} {
		repo := &fakeRepository{}
		w := httptest.NewRecorder()
		router(repo).ServeHTTP(w, httptest.NewRequest("GET", "/foods"+tc.query, nil))
		if w.Code != tc.want {
			t.Fatalf("%s: status %d", tc.query, w.Code)
		}
		if tc.want == 200 && (repo.page != tc.page || repo.size != tc.size || len(repo.ids) != tc.ids) {
			t.Fatalf("incorrect filters: %+v", repo)
		}
	}
}
func TestTagsAndErrors(t *testing.T) {
	for _, tc := range []struct {
		method, path, body string
		err                error
		want               int
	}{
		{"POST", "/tags", `{"name":"Vegan"}`, nil, 201}, {"POST", "/tags", `{"name":" "}`, nil, 400},
		{"POST", "/tags", `{"name":"Vegan"}`, core.ErrConflict, 409}, {"GET", "/tags", "", nil, 200},
		{"POST", "/foods", `{"name":"Rice","long_description":"Dish","photo_url":"photo.jpg","total_calories":1}`, core.ErrInvalidInput, 400},
	} {
		w := httptest.NewRecorder()
		router(&fakeRepository{err: tc.err}).ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}
