package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/taskflow/backend/internal/model"
)

var ErrTaskNotFound = errors.New("task not found")
var ErrTaskConflict = errors.New("task was modified by another request")

type TaskStore struct {
	pool *pgxpool.Pool
}

func NewTaskStore(pool *pgxpool.Pool) *TaskStore {
	return &TaskStore{pool: pool}
}

func (s *TaskStore) Create(ctx context.Context, projectID, createdBy string, req model.CreateTaskRequest) (*model.Task, error) {
	priority := req.Priority
	if priority == "" {
		priority = model.PriorityMedium
	}

	var t model.Task
	err := s.pool.QueryRow(ctx,
		`INSERT INTO tasks (title, description, priority, project_id, assignee_id, created_by, due_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7::date)
		 RETURNING id, title, description, status, priority, project_id, assignee_id, created_by, due_date::text, created_at, updated_at`,
		req.Title, req.Description, priority, projectID, req.AssigneeID, createdBy, req.DueDate,
	).Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.ProjectID, &t.AssigneeID, &t.CreatedBy, &t.DueDate, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}
	return &t, nil
}

func (s *TaskStore) ListByProject(ctx context.Context, projectID string, filter model.TaskFilter) ([]model.Task, error) {
	query := `SELECT id, title, description, status, priority, project_id, assignee_id, created_by, due_date::text, created_at, updated_at
	           FROM tasks WHERE project_id = $1`
	args := []any{projectID}
	argIdx := 2

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.Assignee != nil {
		query += fmt.Sprintf(" AND assignee_id = $%d", argIdx)
		args = append(args, *filter.Assignee)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		offset := 0
		if filter.Page > 1 {
			offset = (filter.Page - 1) * filter.Limit
		}
		args = append(args, filter.Limit, offset)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
			&t.ProjectID, &t.AssigneeID, &t.CreatedBy, &t.DueDate, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *TaskStore) GetByID(ctx context.Context, id string) (*model.Task, error) {
	var t model.Task
	err := s.pool.QueryRow(ctx,
		`SELECT id, title, description, status, priority, project_id, assignee_id, created_by, due_date::text, created_at, updated_at
		 FROM tasks WHERE id = $1`, id,
	).Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.ProjectID, &t.AssigneeID, &t.CreatedBy, &t.DueDate, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	return &t, nil
}

func (s *TaskStore) Update(ctx context.Context, id string, req model.UpdateTaskRequest) (*model.Task, error) {
	var setClauses []string
	var args []any
	argIdx := 1

	if req.Title != nil {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", argIdx))
		args = append(args, *req.Title)
		argIdx++
	}
	if req.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIdx))
		args = append(args, *req.Description)
		argIdx++
	}
	if req.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *req.Status)
		argIdx++
	}
	if req.Priority != nil {
		setClauses = append(setClauses, fmt.Sprintf("priority = $%d", argIdx))
		args = append(args, *req.Priority)
		argIdx++
	}
	if req.AssigneeID != nil {
		setClauses = append(setClauses, fmt.Sprintf("assignee_id = $%d", argIdx))
		args = append(args, *req.AssigneeID)
		argIdx++
	}
	if req.DueDate != nil {
		setClauses = append(setClauses, fmt.Sprintf("due_date = $%d::date", argIdx))
		args = append(args, *req.DueDate)
		argIdx++
	}

	if len(setClauses) == 0 {
		return s.GetByID(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = now()")
	args = append(args, id)

	whereClause := fmt.Sprintf("WHERE id = $%d", argIdx)
	argIdx++

	if req.ExpectedVersion != nil {
		whereClause += fmt.Sprintf(" AND updated_at = $%d::timestamptz", argIdx)
		args = append(args, *req.ExpectedVersion)
	}

	query := fmt.Sprintf(
		`UPDATE tasks SET %s %s
		 RETURNING id, title, description, status, priority, project_id, assignee_id, created_by, due_date::text, created_at, updated_at`,
		strings.Join(setClauses, ", "), whereClause,
	)

	var t model.Task
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.ProjectID, &t.AssigneeID, &t.CreatedBy, &t.DueDate, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if req.ExpectedVersion != nil {
				return nil, ErrTaskConflict
			}
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("update task: %w", err)
	}
	return &t, nil
}

func (s *TaskStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTaskNotFound
	}
	return nil
}

type AssigneeCount struct {
	AssigneeID *string `json:"assignee_id"`
	Name       string  `json:"name"`
	Count      int     `json:"count"`
}

type ProjectStats struct {
	Total      int              `json:"total"`
	ByStatus   map[string]int   `json:"by_status"`
	ByAssignee []AssigneeCount `json:"by_assignee"`
}

func (s *TaskStore) StatsByProject(ctx context.Context, projectID string) (*ProjectStats, error) {
	stats := &ProjectStats{
		ByStatus: map[string]int{
			string(model.StatusTodo):       0,
			string(model.StatusInProgress): 0,
			string(model.StatusDone):       0,
		},
		ByAssignee: []AssigneeCount{},
	}

	rows, err := s.pool.Query(ctx,
		`SELECT status, COUNT(*) FROM tasks WHERE project_id = $1 GROUP BY status`, projectID)
	if err != nil {
		return nil, fmt.Errorf("stats by status: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan status count: %w", err)
		}
		stats.ByStatus[status] = count
		stats.Total += count
	}

	rows2, err := s.pool.Query(ctx,
		`SELECT t.assignee_id, COALESCE(u.name, 'Unassigned'), COUNT(*)
		 FROM tasks t LEFT JOIN users u ON t.assignee_id = u.id
		 WHERE t.project_id = $1
		 GROUP BY t.assignee_id, u.name
		 ORDER BY COUNT(*) DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("stats by assignee: %w", err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var ac AssigneeCount
		if err := rows2.Scan(&ac.AssigneeID, &ac.Name, &ac.Count); err != nil {
			return nil, fmt.Errorf("scan assignee count: %w", err)
		}
		stats.ByAssignee = append(stats.ByAssignee, ac)
	}

	return stats, nil
}
