package pipeline

import "context"

type CloneOptions struct {
	RepoURL     string
	Branch      string
	GithubToken string
}

type Workspace struct {
	Dir       string
	CommitSHA string
	Cleanup   func()
}

func Clone(ctx context.Context, opts CloneOptions) (*Workspace, error) {

}
