package domain

import mealsdomain "backend/internal/modules/meals/domain"

// FoodDayItem uses a date-only string for the calendar date exposed by the API.
type FoodDayItem struct {
	ID          string               `json:"id"`
	ServiceDate string               `json:"service_date"`
	MealType    mealsdomain.MealType `json:"meal_type"`
	FoodID      string               `json:"food_id"`
	Status      FoodDayStatus        `json:"status"`
	Food        Food                 `json:"food"`
}

type FoodDayMeal struct {
	MealType      mealsdomain.MealType `json:"meal_type"`
	TotalCalories float64              `json:"total_calories"`
	Foods         []FoodDayItem        `json:"foods"`
}

type FoodDayGroup struct {
	ServiceDate   string        `json:"service_date"`
	TotalCalories float64       `json:"total_calories"`
	Meals         []FoodDayMeal `json:"meals"`
}
