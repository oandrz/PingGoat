package pipeline

import (
	"PingGoat/internal/database"
	"context"
	"log"
)

func StartWorker(
	ctx context.Context,
	queries *database.Queries,
	id int,
	jobs <-chan JobMessage) {
	for msg := range jobs {
		log.Printf("worker %d processing job: %v", id, msg)
		// pipeline execution
		affectedRows, err := queries.UpdateJob(context.Background(), database.UpdateJobParams{
			Status: string(StatusCloning),
			ID:     msg.JobID,
			UserID: msg.UserId,
		})
		if err != nil {
			log.Printf("failed to update job status: %v", err)
			continue
		}
		if affectedRows == 0 {
			log.Printf("failed to update job status: job not found")
			continue
		}
	}
	log.Printf("worker %d: channel closed, exiting", id)
}
