package handler

import (
	"PingGoat/internal/database"
	"net/http"
)

type authHandler struct {
	queries   *database.Queries
	jwtSecret string
}

type AuthHandler interface {
	Register(w http.ResponseWriter, r *http.Request)
	Login(w http.ResponseWriter, r *http.Request)
}

func NewAuthHandler(queries *database.Queries, jwtSecret string) AuthHandler {
	return &authHandler{
		queries:   queries,
		jwtSecret: jwtSecret,
	}
}

func (h *authHandler) Register(w http.ResponseWriter, r *http.Request) {
	RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Hello, world!",
	})
}

func (h *authHandler) Login(w http.ResponseWriter, r *http.Request) {
	RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Hello, world!",
	})
}
