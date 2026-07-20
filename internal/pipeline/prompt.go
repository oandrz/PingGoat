package pipeline

import (
	"PingGoat/internal/gemini"
	"path/filepath"
	"strings"
)

func selectFiles(files []ParsedFile, doc gemini.DocType) []ParsedFile {
	var out []ParsedFile

	for _, file := range files {
		if matches(file, doc) {
			out = append(out, file)
		}
	}

	return out
}

func matches(p ParsedFile, doc gemini.DocType) bool {
	switch doc {
	case gemini.DocReadme:
		base := filepath.Base(p.Path)
		return base == "go.mod" ||
			base == "package.json" ||
			base == "main.go" ||
			strings.HasPrefix(strings.ToLower(base), "readme") ||
			base == "Dockerfile" || base == "Makefile"
	case gemini.DocQuickStart:
		return strings.Contains(p.Path, "handler") || strings.Contains(p.Path, "route") ||
			strings.Contains(p.Path, "model") || strings.Contains(p.Path, "domain")
	case gemini.DocDiagram:
		return strings.HasSuffix(p.Path, ".go") || strings.HasSuffix(p.Path, ".ts")
	}

	return false
}

func BuildPrompt(files []ParsedFile, doc gemini.DocType) gemini.GenRequest {
	selected := selectFiles(files, doc)

	var b strings.Builder

	switch doc {
	case gemini.DocReadme:
		b.WriteString("You are a technical writer. Generate a professional README.md.\n")
		b.WriteString("Include: title, features, tech stack, installation, project structure.\n")
		b.WriteString("Output only Markdown.\n")
	case gemini.DocQuickStart:
		b.WriteString("You are a developer advocate. Generate a quickstart.md document.\n")
		b.WriteString("Include: prerequisites, steps to run the project or app, working code example, common first task, troubleshooting tips\n")
		b.WriteString("Output only Markdown.\n")
	case gemini.DocDiagram:
		b.WriteString("You are a software architect. Generate a architecture diagram.\n")
		b.WriteString("Include: architecture diagram written using mermaid js on markdown file that shows main component/modules and responsibilities, data flow between components, external dependencies, entry point\n")
		b.WriteString("Output only Markdown. use graph TD or flowchart TD of mermaid\n")
	}

	for _, file := range selected {
		b.WriteString("### ")
		b.WriteString(file.Path)
		b.WriteString("\n")
		// Diagram only needs structure (names + imports), not full source —
		// dumping every file body would blow the token budget.
		if doc == gemini.DocDiagram {
			b.WriteString(extractImports(file.Content))
		} else {
			b.WriteString(file.Content)
		}
		b.WriteString("\n\n")
	}

	return gemini.GenRequest{DocType: doc, Prompt: b.String()}
}

// extractImports returns only the import lines of a Go source file, one per line.
// Used for the diagram prompt, where the dependency structure matters but the
// function bodies do not.
func extractImports(content string) string {
	var b strings.Builder
	inBlock := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "import ("): // block starts
			inBlock = true
		case inBlock && trimmed == ")": // block ends
			inBlock = false
		case inBlock && trimmed != "": // inside block = an import
			b.WriteString(trimmed)
			b.WriteString("\n")
		case strings.HasPrefix(trimmed, "import \""): // single-line import
			b.WriteString(trimmed)
			b.WriteString("\n")
		}
	}

	return b.String()
}
