-- name: CreateJob :one
INSERT INTO jobs (user_id, repo_url, branch)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetJob :one
SELECT * FROM jobs WHERE id = $1 and user_id = $2;

-- name: ListJobsByUser :many
SELECT * FROM jobs
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountJobsByUser :one
SELECT COUNT(*) FROM jobs WHERE user_id = $1;
