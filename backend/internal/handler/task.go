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

type TaskHandler struct {
	tasks    *store.TaskStore
	projects *store.ProjectStore
}

func NewTaskHandler(tasks *store.TaskStore, projects *store.ProjectStore) *TaskHandler {
	return &TaskHandler{tasks: tasks, projects: projects}
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	if _, err := h.projects.GetByID(r.Context(), projectID); err != nil {
		if errors.Is(err, store.ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		slog.Error("get project for task list failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	filter := model.TaskFilter{}
	if s := r.URL.Query().Get("status"); s != "" {
		status := model.TaskStatus(s)
		filter.Status = &status
	}
	if a := r.URL.Query().Get("assignee"); a != "" {
		filter.Assignee = &a
	}
	if p := r.URL.Query().Get("page"); p != "" {
		filter.Page, _ = strconv.Atoi(p)
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		filter.Limit, _ = strconv.Atoi(l)
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}

	tasks, err := h.tasks.ListByProject(r.Context(), projectID, filter)
	if err != nil {
		slog.Error("list tasks failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tasks == nil {
		tasks = []model.Task{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	if _, err := h.projects.GetByID(r.Context(), projectID); err != nil {
		if errors.Is(err, store.ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		slog.Error("get project failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var req model.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if errs := req.Validate(); len(errs) > 0 {
		writeValidationError(w, errs)
		return
	}

	task, err := h.tasks.Create(r.Context(), projectID, userID, req)
	if err != nil {
		slog.Error("create task failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")

	var req model.UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if errs := req.Validate(); len(errs) > 0 {
		writeValidationError(w, errs)
		return
	}

	task, err := h.tasks.Update(r.Context(), taskID, req)
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if errors.Is(err, store.ErrTaskConflict) {
			writeError(w, http.StatusConflict, "task was modified by another request")
			return
		}
		slog.Error("update task failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())

	task, err := h.tasks.GetByID(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, store.ErrTaskNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		slog.Error("get task failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	project, err := h.projects.GetByID(r.Context(), task.ProjectID)
	if err != nil {
		slog.Error("get project for task delete failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if task.CreatedBy != userID && project.OwnerID != userID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.tasks.Delete(r.Context(), taskID); err != nil {
		slog.Error("delete task failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskHandler) Stats(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	if _, err := h.projects.GetByID(r.Context(), projectID); err != nil {
		if errors.Is(err, store.ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		slog.Error("get project for stats failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	stats, err := h.tasks.StatsByProject(r.Context(), projectID)
	if err != nil {
		slog.Error("stats failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}
