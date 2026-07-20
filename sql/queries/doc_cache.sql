-- name: UpsertDocCache :exec
INSERT INTO doc_cache (repo_url, commit_sha, doc_type, content)
VALUES ($1, $2, $3, $4)
ON CONFLICT (repo_url, commit_sha, doc_type)
DO UPDATE SET content    = EXCLUDED.content,
             expires_at = now() + INTERVAL '7 days';
