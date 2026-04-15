package model

import (
	"strings"
	"time"
)

type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
)

type Task struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority"`
	ProjectID   string       `json:"project_id"`
	AssigneeID  *string      `json:"assignee_id"`
	CreatedBy   string       `json:"created_by"`
	DueDate     *string      `json:"due_date,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type CreateTaskRequest struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Priority    TaskPriority `json:"priority"`
	AssigneeID  *string      `json:"assignee_id"`
	DueDate     *string      `json:"due_date"`
}

func (r CreateTaskRequest) Validate() map[string]string {
	errs := make(map[string]string)
	if r.Title == "" {
		errs["title"] = "is required"
	}
	if r.Priority != "" && r.Priority != PriorityLow && r.Priority != PriorityMedium && r.Priority != PriorityHigh {
		errs["priority"] = "must be low, medium, or high"
	}
	return errs
}

type UpdateTaskRequest struct {
	Title           *string       `json:"title"`
	Description     *string       `json:"description"`
	Status          *TaskStatus   `json:"status"`
	Priority        *TaskPriority `json:"priority"`
	AssigneeID      *string       `json:"assignee_id"`
	DueDate         *string       `json:"due_date"`
	ExpectedVersion *string       `json:"expected_version"`
}

func (r UpdateTaskRequest) Validate() map[string]string {
	errs := make(map[string]string)
	if r.Title != nil && strings.TrimSpace(*r.Title) == "" {
		errs["title"] = "cannot be empty"
	}
	if r.Status != nil {
		s := *r.Status
		if s != StatusTodo && s != StatusInProgress && s != StatusDone {
			errs["status"] = "must be todo, in_progress, or done"
		}
	}
	if r.Priority != nil {
		p := *r.Priority
		if p != PriorityLow && p != PriorityMedium && p != PriorityHigh {
			errs["priority"] = "must be low, medium, or high"
		}
	}
	return errs
}

type TaskFilter struct {
	Status   *TaskStatus
	Assignee *string
	Page     int
	Limit    int
}
