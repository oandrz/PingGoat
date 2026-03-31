-- +goose Up
CREATE TABLE doc_cache
(
    id         uuid Primary Key default gen_random_uuid(),
    repo_url   text        not null,
    commit_sha varchar(40) not null,
    doc_type   varchar(20) not null,
    content    text not null,
    created_at timestamptz default now(),
    expires_at timestamptz default now() + INTERVAL '7 days'
);

CREATE UNIQUE INDEX idx_cache_lookup on doc_cache (repo_url, commit_sha, doc_type);

-- +goose Down
DROP TABLE IF EXISTS doc_cache;
