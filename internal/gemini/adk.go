package gemini

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

type adkGenerator struct {
	llm     model.LLM
	limiter *RateLimiter
}

func NewAdkGenerator(ctx context.Context, apiKey, modelID string, limiter *RateLimiter) (Generator, error) {
	llm, err := gemini.NewModel(ctx, modelID, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("create gemini model: %w", err)
	}

	return &adkGenerator{llm: llm, limiter: limiter}, nil
}

func (g *adkGenerator) Generate(ctx context.Context, req GenRequest) (GenResult, error) {
	if err := g.limiter.Wait(ctx); err != nil {
		return GenResult{}, err
	}

	llmReq := &model.LLMRequest{
		Contents: []*genai.Content{{Parts: []*genai.Part{{Text: req.Prompt}}}},
	}

	for resp, err := range g.llm.GenerateContent(ctx, llmReq, false) {
		if err != nil {
			return GenResult{}, fmt.Errorf("gemini generate: %w", err)
		}

		if resp.Content == nil || len(resp.Content.Parts) == 0 {
			return GenResult{}, fmt.Errorf("gemini returned empty response")
		}

		var text strings.Builder
		for _, part := range resp.Content.Parts {
			text.WriteString(part.Text)
		}

		result := GenResult{Content: text.String()}
		if resp.UsageMetadata != nil {
			result.PromptTokens = int(resp.UsageMetadata.PromptTokenCount)
			result.CompletionTokens = int(resp.UsageMetadata.CandidatesTokenCount)
		}

		return result, nil
	}
	return GenResult{}, fmt.Errorf("not implemented")
}
