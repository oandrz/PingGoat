package main

import (
	"PingGoat/internal/config"
	"PingGoat/internal/database"
	"PingGoat/internal/handler"
	"PingGoat/internal/middleware"
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		panic(fmt.Errorf("failed to connect to database: %w", err))
	}
	defer pool.Close()

	dbQueries := database.New(pool)

	authHandler := handler.NewAuthHandler(dbQueries, cfg.JWTSecret, cfg.JWTExpiryHours)
	jobsHandler := handler.NewJobsHandler(dbQueries)

	r := chi.NewRouter()
	/**
	  │  Segment  │                                                                                                                          Why                                                                                                                          │
	  ├───────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
	  │ /api      │ Separates API routes from other things the server might serve (like a health check at /health, or static files at /). Tells consumers "this is the programmatic interface."                                                                           │
	  ├───────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
	  │ /v1       │ API versioning. If you ever need to change the response format or behavior in a breaking way, you create /v2 routes alongside /v1. Existing clients keep working on /v1 while new clients use /v2. Without this, any breaking change breaks everyone. │
	  ├───────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
	  │ /auth     │ Resource grouping. Groups all authentication-related endpoints together. Later you'll have /jobs, /docs, etc.                                                                                                                                         │
	  ├───────────┼───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
	  │ /register │ The action. What this specific endpoint does.
	*/
	r.Post("/api/v1/auth/register", authHandler.Register)
	r.Post("/api/v1/auth/login", authHandler.Login)

	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTAuth(cfg.JWTSecret))
		r.Get("/api/v1/home", authHandler.App)
		r.Post("/api/v1/jobs", jobsHandler.SubmitJob)
		r.Get("/api/v1/jobs", jobsHandler.ListJobs)
		r.Get("/api/v1/jobs/{id}", jobsHandler.GetJobById)
	})

	log.Printf("Serving on: http://localhost:%s/app/\n", cfg.APIPort)
	log.Fatal(http.ListenAndServe(":"+cfg.APIPort, r))
}
