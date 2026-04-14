package handler

import (
	"PingGoat/internal/database"
	"PingGoat/internal/httputil"
	"PingGoat/internal/middleware"
	"PingGoat/internal/pipeline"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const githubPrefix = "https://github.com/"
const defaultRepoBranch = "main"

type jobsHandler struct {
	queries *database.Queries
	jobCh   chan<- pipeline.JobMessage
}

type JobsHandler interface {
	SubmitJob(w http.ResponseWriter, r *http.Request)
	ListJobs(w http.ResponseWriter, r *http.Request)
	GetJobById(w http.ResponseWriter, r *http.Request)
	RemoveJobById(w http.ResponseWriter, r *http.Request)
}

func NewJobsHandler(queries *database.Queries, jobCh chan<- pipeline.JobMessage) JobsHandler {
	return &jobsHandler{
		queries: queries,
		jobCh:   jobCh,
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
	err := httputil.GetUserID(r, &pgUUID, middleware.UserIDKey)
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

	select {
	case h.jobCh <- pipeline.JobMessage{
		JobID:   job.ID,
		RepoURL: params.RepoUrl,
		Branch:  branchName,
	}:
	default:
		log.Printf("job channel full, job %s will be picked up by recovery sweep", job.ID)
	}

	httputil.RespondWithJSON(w, http.StatusAccepted, response{
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
		RepoURL   string `json:"repo_url"`
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
	err = httputil.GetUserID(r, &pgUUID, middleware.UserIDKey)
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
		httputil.RespondWithJSON(w, http.StatusOK, response{
			Data: []jobResponse{},
			Meta: responseMeta{Page: page, PerPage: perPage, Total: 0},
		})
		return
	}

	maxPage := (int(count) + perPage - 1) / perPage
	if page > maxPage {
		page = maxPage
	}

	offset := (page - 1) * perPage
	list, err := h.queries.ListJobsByUser(r.Context(), database.ListJobsByUserParams{
		UserID: pgUUID,
		Limit:  int32(perPage),
		Offset: int32(offset),
	})
	if err != nil {
		log.Printf("failed to list jobs: %v", err)
		httputil.RespondWithError(w, http.StatusInternalServerError, "Failed to list jobs")
		return
	}

	jobResponseList := make([]jobResponse, 0, len(list))
	for _, job := range list {
		jobResponseList = append(jobResponseList, jobResponse{
			ID:        job.ID.String(),
			RepoURL:   job.RepoUrl,
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

func (h *jobsHandler) GetJobById(w http.ResponseWriter, r *http.Request) {
	type response struct {
		ID              string  `json:"id"`
		RepoURL         string  `json:"repo_url"`
		Branch          *string `json:"branch"`
		Status          string  `json:"status"`
		FileCount       int     `json:"file_count"`
		GeminiCallsUsed int     `json:"gemini_calls_used"`
		StartedAt       *string `json:"started_at"`
		CompletedAt     *string `json:"completed_at"`
		CommitSha       *string `json:"commit_sha"`
		ErrorMessage    *string `json:"error_message"`
		CreatedAt       string  `json:"created_at"`
	}

	var pgUUID pgtype.UUID
	err := httputil.GetUserID(r, &pgUUID, middleware.UserIDKey)
	if err != nil {
		httputil.RespondWithError(w, http.StatusUnauthorized, "Invalid User")
		return
	}

	jobId := chi.URLParam(r, "id")
	if jobId == "" {
		log.Printf("Job ID is required")
		httputil.RespondWithError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	var jobIdPgUUID pgtype.UUID
	err = jobIdPgUUID.Scan(jobId)
	if err != nil {
		log.Printf("Invalid Job ID: %v", err)
		httputil.RespondWithError(w, http.StatusBadRequest, "Invalid Job ID")
		return
	}

	job, err := h.queries.GetJob(r.Context(), database.GetJobParams{
		ID:     jobIdPgUUID,
		UserID: pgUUID,
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Job not found: %v", err)
			httputil.RespondWithError(w, http.StatusNotFound, "Job not found")
			return
		}
		log.Printf("failed to get job: %v", err)
		httputil.RespondWithError(w, http.StatusInternalServerError, "Failed to get job")
		return
	}

	httputil.RespondWithJSON(w, http.StatusOK, response{
		ID:              job.ID.String(),
		RepoURL:         job.RepoUrl,
		Branch:          httputil.FormatNullableString(job.Branch),
		Status:          job.Status,
		FileCount:       httputil.IntFromInt4(job.FileCount),
		GeminiCallsUsed: httputil.IntFromInt4(job.GeminiCallsUsed),
		CommitSha:       httputil.FormatNullableString(job.CommitSha),
		ErrorMessage:    httputil.FormatNullableString(job.ErrorMessage),
		StartedAt:       httputil.FormatNullableTime(job.StartedAt),
		CompletedAt:     httputil.FormatNullableTime(job.CompletedAt),
		CreatedAt:       job.CreatedAt.Time.Format(time.RFC3339),
	})
}

func (h *jobsHandler) RemoveJobById(w http.ResponseWriter, r *http.Request) {
	var pgUUID pgtype.UUID
	err := httputil.GetUserID(r, &pgUUID, middleware.UserIDKey)
	if err != nil {
		httputil.RespondWithError(w, http.StatusUnauthorized, "Invalid User")
		return
	}

	jobId := chi.URLParam(r, "id")
	if jobId == "" {
		log.Printf("Job ID is required")
		httputil.RespondWithError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	var jobIdPgUUID pgtype.UUID
	err = jobIdPgUUID.Scan(jobId)
	if err != nil {
		log.Printf("Invalid Job ID: %v", err)
		httputil.RespondWithError(w, http.StatusBadRequest, "Invalid Job ID")
		return
	}

	job, err := h.queries.GetJob(r.Context(), database.GetJobParams{
		ID:     jobIdPgUUID,
		UserID: pgUUID,
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("Job not found: %v", err)
			httputil.RespondWithError(w, http.StatusNotFound, "Job not found")
			return
		}

		log.Printf("failed to get job: %v", err)
		httputil.RespondWithError(w, http.StatusInternalServerError, "Failed to get job")
		return
	}

	if isActiveJobStatus(job.Status) {
		log.Printf("Job is running at the moment")
		httputil.RespondWithError(w, http.StatusConflict, "Job is running")
		return
	}

	affectedRows, err := h.queries.DeleteJob(r.Context(), database.DeleteJobParams{
		ID:     jobIdPgUUID,
		UserID: pgUUID,
	})

	if err != nil {
		log.Printf("failed to delete job: %v", err)
		httputil.RespondWithError(w, http.StatusInternalServerError, "Failed to delete job")
		return
	}

	if affectedRows == 0 {
		httputil.RespondWithError(w, http.StatusNotFound, "Job not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func isActiveJobStatus(status string) bool {
	// These are statuses where the pipeline is actively working
	// Deleting mid-flight would cause the worker goroutine to fail
	switch status {
	case "cloning", "parsing", "generating":
		return true
	default:
		return false
	}
}
