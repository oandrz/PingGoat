-- name: CreateJob :one
INSERT INTO jobs (user_id, repo_url, branch)
VALUES ($1, $2, $3)
RETURNING *;
