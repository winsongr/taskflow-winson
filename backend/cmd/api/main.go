package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/taskflow/backend/internal/config"
	"github.com/taskflow/backend/internal/database"
	"github.com/taskflow/backend/internal/handler"
	"github.com/taskflow/backend/internal/middleware"
	"github.com/taskflow/backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if err := database.RunMigrations(cfg.DatabaseURL, "migrations"); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	pool, err := database.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.RunSeed {
		if err := database.RunSeed(pool, "seed/seed.sql"); err != nil {
			slog.Error("failed to run seed", "error", err)
			os.Exit(1)
		}
	}

	userStore := store.NewUserStore(pool)
	projectStore := store.NewProjectStore(pool)
	taskStore := store.NewTaskStore(pool)

	authHandler := handler.NewAuthHandler(userStore, cfg.JWTSecret)
	projectHandler := handler.NewProjectHandler(projectStore, taskStore)
	taskHandler := handler.NewTaskHandler(taskStore, projectStore)
	healthHandler := handler.NewHealthHandler(pool)

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.RequestLogger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(15 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/health", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.JWTSecret))

		r.Route("/projects", func(r chi.Router) {
			r.Get("/", projectHandler.List)
			r.Post("/", projectHandler.Create)
			r.Get("/{id}", projectHandler.Get)
			r.Patch("/{id}", projectHandler.Update)
			r.Delete("/{id}", projectHandler.Delete)
			r.Get("/{id}/tasks", taskHandler.List)
			r.Post("/{id}/tasks", taskHandler.Create)
			r.Get("/{id}/stats", taskHandler.Stats)
		})

		r.Route("/tasks", func(r chi.Router) {
			r.Patch("/{id}", taskHandler.Update)
			r.Delete("/{id}", taskHandler.Delete)
		})
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced shutdown", "error", err)
	}
	slog.Info("server stopped")
}
