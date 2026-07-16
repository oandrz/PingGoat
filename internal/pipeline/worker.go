package pipeline

import (
	"PingGoat/internal/database"
	"context"
	"fmt"
	"log"
)

func StartWorker(
	ctx context.Context,
	queries *database.Queries,
	id int,
	jobs <-chan JobMessage) {
	for msg := range jobs {
		log.Printf("worker %d processing job: %v", id, msg)
		if err := processJob(ctx, queries, msg); err != nil {
			log.Printf("worker %d: failed to process job: %v", id, err)
		}
	}
	log.Printf("worker %d: channel closed, exiting", id)
}

func processJob(ctx context.Context, queries *database.Queries, msg JobMessage) error {
	affectedRows, err := queries.UpdateJob(context.Background(), database.UpdateJobParams{
		Status: string(StatusCloning),
		ID:     msg.JobID,
		UserID: msg.UserId,
	})
	if err != nil {
		log.Printf("failed to update job status: %v", err)
		return fmt.Errorf("failed to update job status: %v", err)
	}
	if affectedRows == 0 {
		log.Printf("failed to update job status: job not found")
		return fmt.Errorf("failed to update job status: job not found")
	}

	ws, err := Clone(ctx, CloneOptions{
		RepoURL: msg.RepoURL,
		Branch:  msg.Branch,
	})
	if err != nil {
		log.Printf("failed to clone repository: %v", err)
		return err
	}

	log.Printf("Success to clone repository")
	defer ws.Cleanup()

	return nil
}
