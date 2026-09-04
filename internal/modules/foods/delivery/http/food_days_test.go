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
	"time"
)

type dayRepository struct{ err error }

func (r dayRepository) CreateFoodDay(context.Context, *domain.FoodDay) error { return r.err }
func (r dayRepository) ListFoodDays(context.Context, string) ([]domain.FoodDayItem, error) {
	return []domain.FoodDayItem{}, r.err
}
func (r dayRepository) DeleteFoodDay(context.Context, string) error             { return r.err }
func (r dayRepository) CloseFoodDays(context.Context, time.Time) (int64, error) { return 0, r.err }

func TestFoodDayEndpoints(t *testing.T) {
	const id = "00000000-0000-0000-0000-000000000001"
	for _, tc := range []struct {
		method, path, body string
		err                error
		want               int
	}{
		{"POST", "/food-days", `{"service_date":"2026-09-10","meal_type":"LUNCH","food_id":"` + id + `"}`, nil, 201},
		{"POST", "/food-days", `{"service_date":"2026-09-09","meal_type":"LUNCH","food_id":"` + id + `"}`, nil, 400},
		{"POST", "/food-days", `{`, nil, 400},
		{"GET", "/food-days", "", nil, 200},
		{"GET", "/food-days?date=bad", "", nil, 400},
		{"DELETE", "/food-days/" + id, "", nil, 204},
		{"DELETE", "/food-days/" + id, "", core.ErrLocked, 409},
		{"DELETE", "/food-days/" + id, "", core.ErrNotFound, 404},
		{"DELETE", "/food-days/bad", "", nil, 400},
	} {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		h := NewFoodDayHandler(application.NewFoodDayService(dayRepository{tc.err}, func() time.Time { return time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC) }))
		router.POST("/food-days", h.Create)
		router.GET("/food-days", h.List)
		router.DELETE("/food-days/:id", h.Delete)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if tc.want == 201 && (!strings.Contains(w.Body.String(), `"status":"OPEN"`) || !strings.Contains(w.Body.String(), `"service_date":"2026-09-10"`)) {
			t.Fatal("wrong creation response")
		}
	}
}
