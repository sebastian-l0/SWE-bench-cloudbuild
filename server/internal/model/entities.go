package model

import "time"

type Run struct {
	ID           string
	Name         string
	Status       string
	Phase        string
	Dataset      string
	OutputDir    string
	TOSBucket    string
	TOSPrefix    string
	Registry     string
	ManifestJSON string
	Error        string
	CreatedAt    time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

type ImageBuild struct {
	ID           string
	RunID        string
	Layer        string
	LocalKey     string
	TargetImage  string
	ContextPath  string
	Dockerfile   string
	DependsOnKey string
	WorkspaceID  string
	PipelineID   string
	Status       string
	LastRunID    string
	Attempts     int
	Error        string
	RawManifest  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type RunAttempt struct {
	ID            string
	ImageBuildID  string
	PipelineRunID string
	Status        string
	LogURL        string
	StartedAt     time.Time
	UpdatedAt     time.Time
	FinishedAt    *time.Time
}

type RunEvent struct {
	ID        string
	RunID     string
	ImageID   string
	Type      string
	Payload   string
	CreatedAt time.Time
}
