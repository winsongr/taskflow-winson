package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/taskflow/backend/internal/model"
)

var ErrProjectNotFound = errors.New("project not found")

type ProjectStore struct {
	pool *pgxpool.Pool
}

func NewProjectStore(pool *pgxpool.Pool) *ProjectStore {
	return &ProjectStore{pool: pool}
}

func (s *ProjectStore) Create(ctx context.Context, name, description, ownerID string) (*model.Project, error) {
	var p model.Project
	err := s.pool.QueryRow(ctx,
		`INSERT INTO projects (name, description, owner_id)
		 VALUES ($1, $2, $3)
		 RETURNING id, name, description, owner_id, created_at`,
		name, description, ownerID,
	).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}
	return &p, nil
}

func (s *ProjectStore) ListByUser(ctx context.Context, userID string, page, limit int) ([]model.Project, error) {
	if limit <= 0 {
		limit = 50
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT p.id, p.name, p.description, p.owner_id, p.created_at
		 FROM projects p
		 LEFT JOIN tasks t ON t.project_id = p.id
		 WHERE p.owner_id = $1 OR t.assignee_id = $1
		 ORDER BY p.created_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]model.Project, 0, limit)
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func (s *ProjectStore) GetByID(ctx context.Context, id string) (*model.Project, error) {
	var p model.Project
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, description, owner_id, created_at FROM projects WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	return &p, nil
}

func (s *ProjectStore) Update(ctx context.Context, id string, name, description *string) (*model.Project, error) {
	var p model.Project
	err := s.pool.QueryRow(ctx,
		`UPDATE projects
		 SET name = COALESCE($2, name),
		     description = COALESCE($3, description)
		 WHERE id = $1
		 RETURNING id, name, description, owner_id, created_at`,
		id, name, description,
	).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("update project: %w", err)
	}
	return &p, nil
}

func (s *ProjectStore) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrProjectNotFound
	}
	return nil
}
