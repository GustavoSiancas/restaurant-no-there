package domain

import (
	"time"

	core "backend/internal/core/domain"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleStaff Role = "staff"
	RoleUser  Role = "user"
)

type User struct {
	core.Entity
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
	Role         Role
	Active       bool
	LastLoginAt  *time.Time
}
