package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"sync"
)

type FileTier int

type ParsedFile struct {
	Path    string
	Tier    FileTier
	Content string
}

func ParseFiles(ctx context.Context, root string, paths []string, workers int) ([]ParsedFile, error) {
	pathCh := make(chan string)

	go func() {
		for _, path := range paths {
			select {
			case pathCh <- path:
			case <-ctx.Done():
				return
			}
		}
		close(pathCh)
	}()

	var wg sync.WaitGroup
	resultCh := make(chan ParsedFile)
	for range workers {
		wg.Go(func() {
			for path := range pathCh {
				data, err := os.ReadFile(path)
				if err != nil {
					continue
				}

				rel, err := filepath.Rel(root, path)
				if err != nil {
					rel = path
				}
				pf := ParsedFile{Path: rel, Content: string(data)}

				select {
				case resultCh <- pf:
				case <-ctx.Done():
					return
				}
			}
		})
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var results []ParsedFile
	for pf := range resultCh {
		results = append(results, pf)
	}

	return results, nil
}
