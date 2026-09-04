package domain

import (
	"time"

	coredomain "backend/internal/core/domain"
	mealsdomain "backend/internal/modules/meals/domain"
)

type FoodDayStatus string

const (
	FoodDayOpen   FoodDayStatus = "OPEN"
	FoodDayClosed FoodDayStatus = "CLOSED"
)

// FoodDay assigns a food to a meal type on a calendar date.
// Multiple assignments are allowed for the same date, meal type, and food.
type FoodDay struct {
	coredomain.Entity
	ServiceDate time.Time            `json:"service_date"`
	MealType    mealsdomain.MealType `json:"meal_type"`
	FoodID      string               `json:"food_id"`
	Status      FoodDayStatus        `json:"status"`
}
