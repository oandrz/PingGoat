package handler

import (
	"PingGoat/internal/database"
	"PingGoat/internal/httputil"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const pgUniqueViolation = "23505"

// Pre-computed dummy hash for timing-safe login (prevents timing side-channel on user-not-found path)
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

type authHandler struct {
	queries        *database.Queries
	jwtSecret      string
	jwtExpiryHours int
}

type AuthHandler interface {
	Register(w http.ResponseWriter, r *http.Request)
	Login(w http.ResponseWriter, r *http.Request)
	App(w http.ResponseWriter, r *http.Request)
}

func NewAuthHandler(queries *database.Queries, jwtSecret string, jwtExpiryHours int) AuthHandler {
	return &authHandler{
		queries:        queries,
		jwtSecret:      jwtSecret,
		jwtExpiryHours: jwtExpiryHours,
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
		httputil.RespondWithError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if params.Email == "" || params.Password == "" {
		httputil.RespondWithError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(params.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("failed to hash password: %v", err)
		httputil.RespondWithError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := h.queries.CreateUser(r.Context(), database.CreateUserParams{
		Email:        params.Email,
		PasswordHash: string(hashedBytes),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			httputil.RespondWithError(w, http.StatusConflict, "email already in use")
			return
		}

		log.Printf("failed to create user: %v", err)
		httputil.RespondWithError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	httputil.RespondWithJSON(w, http.StatusCreated, response{
		ID:        user.ID.String(),
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Time.Format(time.RFC3339),
	})
}

func (h *authHandler) Login(w http.ResponseWriter, r *http.Request) {
	type requestParameter struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	type response struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		CreatedAt string `json:"created_at"`
		JWTToken  string `json:"jwt_token"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var param requestParameter
	if err := json.NewDecoder(r.Body).Decode(&param); err != nil {
		httputil.RespondWithError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if param.Email == "" || param.Password == "" {
		httputil.RespondWithError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := h.queries.GetUserByEmail(r.Context(), param.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// User not found — run dummy bcrypt to match timing of the real path
			_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(param.Password))
			httputil.RespondWithError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}

		log.Printf("database error during login: %v", err)
		httputil.RespondWithError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(param.Password)); err != nil {
		// intended the same with the password one to reduce the chance of attacker go into the app
		httputil.RespondWithError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	claims := jwt.RegisteredClaims{
		Subject:   user.ID.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(h.jwtExpiryHours) * time.Hour)),
	}
	unsignedToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := unsignedToken.SignedString([]byte(h.jwtSecret))
	if err != nil {
		log.Printf("failed to sign token: %v", err)
		httputil.RespondWithError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}

	httputil.RespondWithJSON(w, http.StatusOK, response{
		ID:        user.ID.String(),
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Time.Format(time.RFC3339),
		JWTToken:  signedToken,
	})
}

func (h *authHandler) App(w http.ResponseWriter, r *http.Request) {
	httputil.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Hello, world!"})
}
