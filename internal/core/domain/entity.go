package domain

import "time"

// Entity contains the fields shared by persisted domain entities.
type Entity struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}
