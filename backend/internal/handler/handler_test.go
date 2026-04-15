package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/taskflow/backend/internal/database"
	"github.com/taskflow/backend/internal/handler"
	"github.com/taskflow/backend/internal/middleware"
	"github.com/taskflow/backend/internal/store"
)

var testRouter *chi.Mux

func TestMain(m *testing.M) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://taskflow:taskflow@localhost:5432/taskflow?sslmode=disable"
	}
	jwtSecret := "test-secret"

	pool, err := database.NewPool(context.Background(), dbURL)
	if err != nil {
		panic("cannot connect to test db: " + err.Error())
	}
	defer pool.Close()

	pool.Exec(context.Background(), "DELETE FROM tasks")
	pool.Exec(context.Background(), "DELETE FROM projects")
	pool.Exec(context.Background(), "DELETE FROM users")

	userStore := store.NewUserStore(pool)
	projectStore := store.NewProjectStore(pool)
	taskStore := store.NewTaskStore(pool)

	authHandler := handler.NewAuthHandler(userStore, jwtSecret)
	projectHandler := handler.NewProjectHandler(projectStore, taskStore)
	taskHandler := handler.NewTaskHandler(taskStore, projectStore)

	testRouter = chi.NewRouter()
	testRouter.Route("/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
	})
	testRouter.Group(func(r chi.Router) {
		r.Use(middleware.Auth(jwtSecret))
		r.Route("/projects", func(r chi.Router) {
			r.Get("/", projectHandler.List)
			r.Post("/", projectHandler.Create)
			r.Get("/{id}", projectHandler.Get)
			r.Get("/{id}/tasks", taskHandler.List)
			r.Post("/{id}/tasks", taskHandler.Create)
		})
		r.Route("/tasks", func(r chi.Router) {
			r.Patch("/{id}", taskHandler.Update)
		})
	})

	os.Exit(m.Run())
}

func doRequest(method, path string, body any, token string) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = &bytes.Buffer{}
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w
}

func parseJSON(w *httptest.ResponseRecorder) map[string]any {
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	return result
}

func TestRegisterAndLoginFlow(t *testing.T) {
	w := doRequest("POST", "/auth/register", map[string]string{
		"name": "Test User", "email": "flow@test.com", "password": "password123",
	}, "")

	if w.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	resp := parseJSON(w)
	if resp["token"] == nil || resp["token"] == "" {
		t.Fatal("register: missing token")
	}
	user := resp["user"].(map[string]any)
	if user["email"] != "flow@test.com" {
		t.Fatalf("register: email mismatch: %v", user["email"])
	}

	w2 := doRequest("POST", "/auth/login", map[string]string{
		"email": "flow@test.com", "password": "password123",
	}, "")
	if w2.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", w2.Code)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	doRequest("POST", "/auth/register", map[string]string{
		"name": "Wrong PW", "email": "wrong@test.com", "password": "password123",
	}, "")

	w := doRequest("POST", "/auth/login", map[string]string{
		"email": "wrong@test.com", "password": "wrongpassword",
	}, "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: expected 401, got %d", w.Code)
	}
	resp := parseJSON(w)
	if resp["error"] != "invalid credentials" {
		t.Fatalf("wrong password: expected 'invalid credentials', got %v", resp["error"])
	}
}

func TestProjectCRUDWithAuth(t *testing.T) {
	w := doRequest("POST", "/auth/register", map[string]string{
		"name": "Project Owner", "email": "owner@test.com", "password": "password123",
	}, "")
	token := parseJSON(w)["token"].(string)

	wNoAuth := doRequest("GET", "/projects", nil, "")
	if wNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("unauth: expected 401, got %d", wNoAuth.Code)
	}

	wCreate := doRequest("POST", "/projects", map[string]string{
		"name": "Test Project", "description": "Integration test",
	}, token)
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("create project: expected 201, got %d: %s", wCreate.Code, wCreate.Body.String())
	}
	project := parseJSON(wCreate)
	projectID := project["id"].(string)

	wGet := doRequest("GET", "/projects/"+projectID, nil, token)
	if wGet.Code != http.StatusOK {
		t.Fatalf("get project: expected 200, got %d", wGet.Code)
	}
	getResp := parseJSON(wGet)
	if getResp["name"] != "Test Project" {
		t.Fatalf("get project: name mismatch: %v", getResp["name"])
	}
	tasks := getResp["tasks"].([]any)
	if len(tasks) != 0 {
		t.Fatalf("get project: expected 0 tasks, got %d", len(tasks))
	}

	wList := doRequest("GET", "/projects", nil, token)
	if wList.Code != http.StatusOK {
		t.Fatalf("list projects: expected 200, got %d", wList.Code)
	}
	listResp := parseJSON(wList)
	projects := listResp["projects"].([]any)
	if len(projects) == 0 {
		t.Fatal("list projects: expected at least 1 project")
	}

	wNotFound := doRequest("GET", "/projects/00000000-0000-0000-0000-000000000000", nil, token)
	if wNotFound.Code != http.StatusNotFound {
		t.Fatalf("not found: expected 404, got %d", wNotFound.Code)
	}
}
