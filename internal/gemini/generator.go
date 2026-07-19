package gemini

import "context"

type DocType string

const (
	DocReadme     DocType = "readme"
	DocQuickStart DocType = "quickstart"
	DocDiagram    DocType = "diagram"
)

type GenRequest struct {
	DocType DocType
	Prompt  string
}

type GenResult struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
}

type Generator interface {
	Generate(ctx context.Context, request GenRequest) (GenResult, error)
}
