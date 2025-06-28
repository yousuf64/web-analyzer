package models

import (
	"time"
)

// Job represents an analysis job domain model
type Job struct {
	ID          string
	URL         string
	Status      JobStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Result      *AnalyzeResult
}

// JobStatus represents the overall status of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// Task represents an individual task within a job
type Task struct {
	JobID    string
	Type     TaskType
	Status   TaskStatus
	SubTasks map[string]SubTask
}

// TaskType represents different types of analysis tasks
type TaskType string

const (
	TaskTypeExtracting         TaskType = "extracting"
	TaskTypeIdentifyingVersion TaskType = "identifying_version"
	TaskTypeAnalyzing          TaskType = "analyzing"
	TaskTypeVerifyingLinks     TaskType = "verifying_links"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusSkipped   TaskStatus = "skipped"
)

// SubTask represents a subtask within a task
type SubTask struct {
	Type        SubTaskType
	Status      TaskStatus
	URL         string
	Description string
}

// SubTaskType represents the type of a subtask
type SubTaskType string

const (
	SubTaskTypeValidatingLink SubTaskType = "validating_link"
)

// AnalyzeResult represents the result of an analysis
type AnalyzeResult struct {
	HtmlVersion       string
	PageTitle         string
	Headings          map[string]int
	Links             []string
	InternalLinkCount int
	ExternalLinkCount int
	AccessibleLinks   int
	InaccessibleLinks int
	HasLoginForm      bool
}
