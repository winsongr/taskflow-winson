package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/taskflow/backend/internal/middleware"
	"github.com/taskflow/backend/internal/model"
	"github.com/taskflow/backend/internal/store"
)

type ProjectHandler struct {
	projects *store.ProjectStore
	tasks    *store.TaskStore
}

func NewProjectHandler(projects *store.ProjectStore, tasks *store.TaskStore) *ProjectHandler {
	return &ProjectHandler{projects: projects, tasks: tasks}
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if page <= 0 {
		page = 1
	}

	projects, err := h.projects.ListByUser(r.Context(), userID, page, limit)
	if err != nil {
		slog.Error("list projects failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"projects": projects,
		"page":     page,
		"limit":    limit,
	})
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if errs := req.Validate(); len(errs) > 0 {
		writeValidationError(w, errs)
		return
	}

	userID := middleware.GetUserID(r.Context())
	project, err := h.projects.Create(r.Context(), req.Name, req.Description, userID)
	if err != nil {
		slog.Error("create project failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	project, err := h.projects.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		slog.Error("get project failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	tasks, err := h.tasks.ListByProject(r.Context(), id, model.TaskFilter{})
	if err != nil {
		slog.Error("list tasks for project failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tasks == nil {
		tasks = []model.Task{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          project.ID,
		"name":        project.Name,
		"description": project.Description,
		"owner_id":    project.OwnerID,
		"created_at":  project.CreatedAt,
		"tasks":       tasks,
	})
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	existing, err := h.projects.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		slog.Error("get project failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing.OwnerID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req model.UpdateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if errs := req.Validate(); len(errs) > 0 {
		writeValidationError(w, errs)
		return
	}

	project, err := h.projects.Update(r.Context(), id, req.Name, req.Description)
	if err != nil {
		slog.Error("update project failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	existing, err := h.projects.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		slog.Error("get project failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing.OwnerID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.projects.Delete(r.Context(), id); err != nil {
		slog.Error("delete project failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
