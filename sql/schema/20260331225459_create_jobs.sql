-- +goose Up
CREATE TABLE jobs (
    id uuid Primary Key default gen_random_uuid(),
    user_id uuid not null,
    foreign key (user_id) references users(id) on delete cascade,
    repo_url text not null,
    branch varchar(255) default 'main',
    commit_sha varchar(40),
    status varchar(20) not null default 'queued',
    error_message text,
    file_count int,
    gemini_calls_used int,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

CREATE INDEX idx_user_created_at on jobs (user_id, created_at DESC);
CREATE INDEX idx_worker_status on jobs (status);

-- +goose Down
DROP TABLE IF EXISTS jobs;
