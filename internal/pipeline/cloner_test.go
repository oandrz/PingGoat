package pipeline

import (
	"os"
	"strings"
	"testing"
)

func TestValidateRepoURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://github.com/octocat/Hello-World", false},
		{"valid with .git", "https://github.com/octocat/Hello-World.git", false},
		{"http not https", "http://github.com/a/b", true},
		{"wrong host", "https://gitlab.com/a/b", true},
		{"file scheme", "file:///etc/passwd", true},
		{"path traversal", "https://github.com/../../etc", true},
		{"empty", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRepoURL(tc.url)
			if (err != nil) != tc.wantErr {
				t.Fatalf("got err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestWorkspaceCleanupIdempotent(t *testing.T) {
	dir, _ := os.MkdirTemp("", "docgoat-test-*")
	ws := &Workspace{Dir: dir, Cleanup: func() { os.RemoveAll(dir) }}
	ws.Cleanup()
	ws.Cleanup()
}

func TestRedact(t *testing.T) {
	in := []string{"-c", "http.extraHeader=Authorization: Bearer sekret", "clone"}
	out := redact(in)
	for _, a := range out {
		if strings.Contains(a, "sekret") {
			t.Fatal("token leaked in redacted output")
		}
	}
}
