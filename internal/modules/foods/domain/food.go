package domain

import coredomain "backend/internal/core/domain"

// Food describes a food item and its assigned tags.
type Food struct {
	coredomain.Entity
	Name            string  `json:"name"`
	LongDescription string  `json:"long_description"`
	TotalCalories   float64 `json:"total_calories"`
	PhotoURL        string  `json:"photo_url"`
	Tags            []Tag   `json:"tags"`
}
