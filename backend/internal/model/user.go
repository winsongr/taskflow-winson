package model

import (
	"net/mail"
	"strings"
	"time"
)

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r RegisterRequest) Validate() map[string]string {
	errs := make(map[string]string)
	if strings.TrimSpace(r.Name) == "" {
		errs["name"] = "is required"
	}
	if r.Email == "" {
		errs["email"] = "is required"
	} else if _, err := mail.ParseAddress(r.Email); err != nil {
		errs["email"] = "must be a valid email"
	}
	if r.Password == "" {
		errs["password"] = "is required"
	} else if len(r.Password) < 8 {
		errs["password"] = "must be at least 8 characters"
	}
	return errs
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r LoginRequest) Validate() map[string]string {
	errs := make(map[string]string)
	if r.Email == "" {
		errs["email"] = "is required"
	}
	if r.Password == "" {
		errs["password"] = "is required"
	}
	return errs
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
