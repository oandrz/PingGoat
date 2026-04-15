package pipeline

import (
	"PingGoat/internal/database"
	"context"
	"log"
	"time"
)

func StartRecoverySweep(
	ctx context.Context,
	queries *database.Queries,
	jobs chan<- JobMessage,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pendingJobs, err := queries.GetPendingJob(ctx)
			if err != nil {
				log.Printf("Error getting pending jobs: %v", err)
				continue
			}

			for _, job := range pendingJobs {
				branch := ""
				if job.Branch.Valid {
					branch = job.Branch.String
				}
				select {
				case jobs <- JobMessage{
					JobID:   job.ID,
					RepoURL: job.RepoUrl,
					Branch:  branch,
					UserId:  job.UserID,
				}:
				default:
					log.Printf("Job channel full, job %s will be picked up by recovery sweep", job.ID)
				}
			}
		}
	}
}
