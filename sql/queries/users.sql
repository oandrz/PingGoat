-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: UpdateGithubToken :exec
UPDATE users
SET github_token_enc = $2, updated_at = now()
WHERE id = $1;

-- name: DeleteGithubToken :exec
UPDATE users
SET github_token_enc = NULL, updated_at = now()
WHERE id = $1;
