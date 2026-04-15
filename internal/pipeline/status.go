package pipeline

type JobStatus string

const (
	StatusQueued     JobStatus = "queued"
	StatusCloning    JobStatus = "cloning"
	StatusParsing    JobStatus = "parsing"
	StatusGenerating JobStatus = "generating"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

func (s JobStatus) IsActive() bool {
	switch s {
	case StatusCloning, StatusParsing, StatusGenerating:
		return true
	default:
		return false
	}
}
