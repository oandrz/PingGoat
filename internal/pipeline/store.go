package pipeline

import (
	"PingGoat/internal/database"
	"PingGoat/internal/gemini"
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// StoredDoc pairs a generated document with its type. The generate loop
// produces plain gemini.GenResult values, which carry content and token
// counts but NOT which doc they are — so we tag each one here, otherwise
// the store stage can't tell the readme from the diagram.
type StoredDoc struct {
	DocType gemini.DocType
	Result  gemini.GenResult
}

// StoreInput is everything the store stage needs to persist a finished job.
type StoreInput struct {
	JobID           pgtype.UUID
	UserID          pgtype.UUID
	RepoURL         string
	CommitSHA       string
	FileCount       int32
	GeminiCallsUsed int32
	Docs            []StoredDoc
}

// StoreResults persists a completed job atomically. Every document row,
// every cache row, and the job's flip to "completed" happen inside ONE
// transaction: if any single write fails, the deferred Rollback undoes
// all of them and the job is left in its previous state for the recovery
// sweep to retry. The job only shows "completed" once Commit succeeds.
func StoreResults(ctx context.Context, pool *pgxpool.Pool, queries *database.Queries, in StoreInput) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	// Rollback is a no-op after a successful Commit; on any early return
	// it undoes partial writes. This line is your safety net — keep it.
	defer tx.Rollback(ctx)

	// qtx runs the SAME queries but bound to the transaction. Use qtx for
	// every write below — never `queries`, or the write escapes the tx.
	qtx := queries.WithTx(tx)

	for _, doc := range in.Docs {
		err := qtx.UpsertDocument(
			ctx,
			database.UpsertDocumentParams{
				JobID:            in.JobID,
				DocType:          string(doc.DocType),
				Content:          doc.Result.Content,
				PromptTokens:     pgtype.Int4{Int32: int32(doc.Result.PromptTokens), Valid: true},
				CompletionTokens: pgtype.Int4{Int32: int32(doc.Result.CompletionTokens), Valid: true},
			})
		if err != nil {
			return fmt.Errorf("store: upsert doc: %w", err)
		}

		err = qtx.UpsertDocCache(ctx, database.UpsertDocCacheParams{
			RepoUrl:   in.RepoURL,
			CommitSha: in.CommitSHA,
			DocType:   string(doc.DocType),
			Content:   doc.Result.Content,
		})
		if err != nil {
			return fmt.Errorf("store: upsert doc cache: %w", err)
		}
	}

	rows, err := qtx.CompleteJob(ctx, database.CompleteJobParams{
		CommitSha:       pgtype.Text{String: in.CommitSHA, Valid: true},
		FileCount:       pgtype.Int4{Int32: in.FileCount, Valid: true},
		GeminiCallsUsed: pgtype.Int4{Int32: in.GeminiCallsUsed, Valid: true},
		ID:              in.JobID,
		UserID:          in.UserID,
	})
	if err != nil {
		return fmt.Errorf("store: complete job: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("store: empty results")
	}

	return tx.Commit(ctx)
}
