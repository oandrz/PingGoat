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

-- name: GetPendingJob :many
SELECT * FROM jobs
WHERE status = 'queued'
LIMIT 50 ORDER BY created_at ASC ;

-- name: CountJobsByUser :one
SELECT COUNT(*) FROM jobs WHERE user_id = $1;

-- name: DeleteJob :execrows
DELETE FROM jobs WHERE id = $1 and user_id = $2;

-- name: UpdateJob :execrows
UPDATE jobs SET status = $1, updated_at = now() WHERE id = $2 and user_id = $3;