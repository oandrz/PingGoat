package handler

import (
	"PingGoat/internal/database"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const pgUniqueViolation = "23505"

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
	type requestParameter struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	type response struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		CreatedAt string `json:"created_at"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	var params requestParameter
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if params.Email == "" || params.Password == "" {
		RespondWithError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("failed to hash password: %v", err)
		RespondWithError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := h.queries.CreateUser(r.Context(), database.CreateUserParams{
		Email:        params.Email,
		PasswordHash: string(hashedBytes),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			RespondWithError(w, http.StatusConflict, "email already in use")
			return
		}

		log.Printf("failed to create user: %v", err)
		RespondWithError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	RespondWithJSON(w, http.StatusCreated, response{
		ID:        user.ID.String(),
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Time.Format(time.RFC3339),
	})
}

func (h *authHandler) Login(w http.ResponseWriter, r *http.Request) {
	RespondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Hello, world!",
	})
}
