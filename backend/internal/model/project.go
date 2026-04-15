package model

import (
	"strings"
	"time"
)

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	Tasks       []Task    `json:"tasks,omitempty"`
}

type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (r CreateProjectRequest) Validate() map[string]string {
	errs := make(map[string]string)
	if r.Name == "" {
		errs["name"] = "is required"
	}
	return errs
}

type UpdateProjectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (r UpdateProjectRequest) Validate() map[string]string {
	errs := make(map[string]string)
	if r.Name != nil && strings.TrimSpace(*r.Name) == "" {
		errs["name"] = "cannot be empty"
	}
	return errs
}
