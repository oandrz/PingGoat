-- name: UpsertDocument :exec
INSERT INTO documents (job_id, doc_type, content, prompt_tokens, completion_tokens)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (job_id, doc_type)
DO UPDATE SET content           = EXCLUDED.content,
             prompt_tokens     = EXCLUDED.prompt_tokens,
             completion_tokens = EXCLUDED.completion_tokens;
