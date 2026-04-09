package handler

import (
	"PingGoat/internal/database"
	"PingGoat/internal/httputil"
	"PingGoat/internal/middleware"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
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
	ListJobs(w http.ResponseWriter, r *http.Request)
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

	var pgUUID pgtype.UUID
	err := httputil.GetUserId(r, &pgUUID, middleware.UserIDKey)
	if err != nil {
		httputil.RespondWithError(w, http.StatusUnauthorized, "Invalid User")
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

func (h *jobsHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	type jobResponse struct {
		ID        string `json:"id"`
		RepoUrl   string `json:"repo_url"`
		Branch    string `json:"branch"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}

	type responseMeta struct {
		Page    int `json:"page"`
		PerPage int `json:"per_page"`
		Total   int `json:"total"`
	}

	type response struct {
		Data []jobResponse `json:"data"`
		Meta responseMeta  `json:"meta"`
	}

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	perPage, err := strconv.Atoi(r.URL.Query().Get("per_page"))
	if err != nil || perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	var pgUUID pgtype.UUID
	err = httputil.GetUserId(r, &pgUUID, middleware.UserIDKey)
	if err != nil {
		httputil.RespondWithError(w, http.StatusUnauthorized, "Invalid User")
		return
	}

	count, err := h.queries.CountJobsByUser(r.Context(), pgUUID)
	if err != nil {
		log.Printf("failed to count jobs: %v", err)
		httputil.RespondWithError(w, http.StatusInternalServerError, "Failed to list jobs")
		return
	}

	if count == 0 {
		httputil.RespondWithJSON(w, http.StatusOK, response{})
		return
	}

	offSet := (page - 1) * perPage
	list, err := h.queries.ListJobsByUser(r.Context(), database.ListJobsByUserParams{
		UserID: pgUUID,
		Limit:  int32(perPage),
		Offset: int32(offSet),
	})
	if err != nil {
		log.Printf("failed to list jobs: %v", err)
		httputil.RespondWithError(w, http.StatusInternalServerError, "Failed to list jobs")
		return
	}

	if len(list) == 0 {
		httputil.RespondWithJSON(w, http.StatusOK, response{})
		return
	}

	var jobResponseList []jobResponse
	for _, job := range list {
		jobResponseList = append(jobResponseList, jobResponse{
			ID:        job.ID.String(),
			RepoUrl:   job.RepoUrl,
			Branch:    job.Branch.String,
			Status:    job.Status,
			CreatedAt: job.CreatedAt.Time.Format(time.RFC3339),
		})
	}
	httputil.RespondWithJSON(w, http.StatusOK, response{
		Data: jobResponseList,
		Meta: responseMeta{
			Page:    page,
			PerPage: perPage,
			Total:   int(count),
		},
	})
}
