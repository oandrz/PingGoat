-- +goose Up
CREATE TABLE documents (
    id uuid Primary Key default gen_random_uuid(),
    job_id uuid not null,
    foreign key (job_id) references jobs(id) on delete cascade,
    doc_type varchar(20) not null,
    content text not null,
    prompt_tokens int,
    completion_tokens int,
    created_at timestamptz default now()
);

CREATE UNIQUE INDEX idx_documents_job_type on documents (job_id, doc_type);

-- +goose Down
DROP TABLE IF EXISTS documents;
