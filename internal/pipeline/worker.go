package pipeline

import (
	"PingGoat/internal/config"
	"PingGoat/internal/database"
	"PingGoat/internal/gemini"
	"context"
	"fmt"
	"log"
)

func StartWorker(
	ctx context.Context,
	queries *database.Queries,
	id int,
	jobs <-chan JobMessage,
	cfg config.Config,
) {
	for msg := range jobs {
		log.Printf("worker %d processing job: %v", id, msg)
		if err := processJob(ctx, queries, msg, cfg); err != nil {
			log.Printf("worker %d: failed to process job: %v", id, err)
		}
	}
	log.Printf("worker %d: channel closed, exiting", id)
}

func processJob(ctx context.Context, queries *database.Queries, msg JobMessage, cfg config.Config) error {
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

	affectedRows, err = queries.UpdateJob(context.Background(), database.UpdateJobParams{
		Status: string(StatusParsing),
		ID:     msg.JobID,
		UserID: msg.UserId,
	})
	if err != nil {
		log.Printf("failed to update job status into parse: %v", err)
		return fmt.Errorf("failed to update job status into parse: %v", err)
	}
	if affectedRows == 0 {
		log.Printf("failed to update job status into parse: job not found")
		return fmt.Errorf("failed to update job status into parse: job not found")
	}

	paths, err := ScanFiles(ws.Dir, cfg.MaxFilesPerRepo)
	if err != nil {
		log.Printf("failed to scan files: %v", err)
		return err
	}

	parsedFiles, err := ParseFiles(ctx, ws.Dir, paths, cfg.PipelineWorkers)
	if err != nil {
		log.Printf("failed to parse files: %v", err)
		return err
	}

	log.Printf("Parse process successful")

	batches := BatchFiles(parsedFiles, cfg.MaxTokensPerBatch)
	if len(batches) == 0 {
		log.Printf("no parseable files in repo")
		return fmt.Errorf("no parseable files in repo")
	}

	log.Printf("packed %d files into %d batches", len(parsedFiles), len(batches))

	var gen gemini.Generator
	if cfg.GeminiAPIKey != "" {
		limiter := gemini.NewRateLimiter(cfg.GeminiRPM)
		defer limiter.Stop()
		gen, err = gemini.NewAdkGenerator(ctx, cfg.GeminiAPIKey, cfg.GeminiModel, limiter)
		if err != nil {
			return err
		}
	}

	affectedRows, err = queries.UpdateJob(context.Background(), database.UpdateJobParams{
		Status: string(StatusGenerating),
		ID:     msg.JobID,
		UserID: msg.UserId,
	})
	if err != nil {
		log.Printf("failed to update job status into generating: %v", err)
		return fmt.Errorf("failed to update job status into generating: %v", err)
	}
	if affectedRows == 0 {
		log.Printf("failed to update job status into generating: job not found")
		return fmt.Errorf("failed to update job status into generating: job not found")
	}

	docTypes := []gemini.DocType{gemini.DocReadme, gemini.DocQuickStart, gemini.DocDiagram}
	var docs []gemini.GenResult
	for _, dt := range docTypes {
		req := gemini.BuildPrompt(parsedFiles, dt)
		res, genErr := gen.Generate(ctx, req)
		if genErr != nil {
			log.Printf("generate %s failed: %v", dt, genErr)
			continue // independent docs: one failure doesn't kill the job
		}
		docs = append(docs, res)
	}
	log.Printf("generated %d/%d docs", len(docs), len(docTypes))

	return nil
}
