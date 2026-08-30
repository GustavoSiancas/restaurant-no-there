package domain

import "time"

// Entity contains the fields shared by persisted domain entities.
type Entity struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
