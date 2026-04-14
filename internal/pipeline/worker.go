package pipeline

import (
	"log"
)

func StartWorker(id int, jobs <-chan JobMessage) {
	for msg := range jobs {
		log.Printf("worker %d processing job: %v", id, msg)
		// pipeline execution
	}
	log.Printf("worker %d: channel closed, exiting", id)
}
