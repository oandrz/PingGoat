package pipeline

import "github.com/jackc/pgx/v5/pgtype"

type JobMessage struct {
	JobID   pgtype.UUID
	RepoURL string
	Branch  string
}
