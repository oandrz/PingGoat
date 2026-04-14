package pipeline

import (
	"context"
	"log"
)

func StartWorker(ctx context.Context, id int, jobs <-chan JobMessage) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("worker %d stopped", id)
			return
		case msg, ok := <-jobs:
			if !ok {
				log.Printf("worker %d stopped", id)
				return
			}
			log.Printf("worker %d processing job: %v", id, msg)
			// run pipeline
		}
	}
}
