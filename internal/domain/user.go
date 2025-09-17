package domain

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	ID             int        `json:"id"`
	Name           string     `json:"name"`
	Lastname       string     `json:"lastname"`
	Email          string     `json:"email"`
	PasswordHash   string     `json:"password"`
	Active         bool       `json:"active"`
	RoleID         int        `json:"role_id"`
	AvatarURL      *string    `json:"avatar_url"`
	Deleted        bool       `json:"deleted"`
	DeletedAt      *time.Time `json:"deleted_at"`
	LinkedAccounts []string   `json:"linked_accounts"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type UpdateUserRequest struct {
	ID        int     `json:"id"`
	Name      *string `json:"name"`
	Lastname  *string `json:"lastname"`
	Email     *string `json:"email"`
	Active    *bool   `json:"active"`
	RoleID    *int    `json:"role_id"`
	AvatarURL *string `json:"avatar_url"`
	Deleted   *bool   `json:"deleted"`
}

type Claims struct {
	UserID        int
	UserName      string
	UserLastname  string
	UserEmail     string
	UserActive    bool
	UserRoleID    int
	UserAvatarURL *string
	UserAccounts  []string
	jwt.RegisteredClaims
}
