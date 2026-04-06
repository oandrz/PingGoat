package handler

import (
	"PingGoat/internal/database"
	"PingGoat/internal/httputil"
	"PingGoat/internal/middleware"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

const githubPrefix = "https://github.com/"
const defaultRepoBranch = "main"

type jobsHandler struct {
	queries *database.Queries
}

type JobsHandler interface {
	SubmitJob(w http.ResponseWriter, r *http.Request)
}

func NewJobsHandler(queries *database.Queries) JobsHandler {
	return &jobsHandler{
		queries: queries,
	}
}

func (h *jobsHandler) SubmitJob(w http.ResponseWriter, r *http.Request) {
	type requestParameter struct {
		RepoUrl string `json:"repo_url"`
		Branch  string `json:"branch"`
	}

	type response struct {
		ID        string `json:"id"`
		RepoUrl   string `json:"repo_url"`
		Branch    string `json:"branch"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var params requestParameter
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		httputil.RespondWithError(w, http.StatusBadRequest, "Invalid Request Body")
		return
	}

	if params.RepoUrl == "" {
		httputil.RespondWithError(w, http.StatusBadRequest, "Repo URL are required")
		return
	}

	if !strings.HasPrefix(params.RepoUrl, githubPrefix) {
		httputil.RespondWithError(w, http.StatusBadRequest, "Could not access repository")
		return
	}

	branchName := defaultRepoBranch
	if params.Branch != "" {
		branchName = params.Branch
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok {
		httputil.RespondWithError(w, http.StatusUnauthorized, "User not found in system")
		return
	}

	var pgUUID pgtype.UUID
	err := pgUUID.Scan(userID)
	if err != nil {
		httputil.RespondWithError(w, http.StatusInternalServerError, "User ID Parsing Error")
		return
	}

	job, err := h.queries.CreateJob(r.Context(), database.CreateJobParams{
		UserID:  pgUUID,
		RepoUrl: params.RepoUrl,
		Branch:  pgtype.Text{String: branchName, Valid: true},
	})
	if err != nil {
		httputil.RespondWithError(w, http.StatusInternalServerError, "Failed to create job")
		return
	}

	httputil.RespondWithJSON(w, http.StatusCreated, response{
		ID:        job.ID.String(),
		RepoUrl:   job.RepoUrl,
		Branch:    job.Branch.String,
		Status:    job.Status,
		CreatedAt: job.CreatedAt.Time.Format(time.RFC3339),
	})
}
