package models

import (
	"time"
)

// Job represents an analysis job domain model
type Job struct {
	ID          string         `json:"id"`
	URL         string         `json:"url"`
	Status      JobStatus      `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	StartedAt   *time.Time     `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at"`
	Result      *AnalyzeResult `json:"result"`
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
	JobID    string             `json:"job_id"`
	Type     TaskType           `json:"type"`
	Status   TaskStatus         `json:"status"`
	SubTasks map[string]SubTask `json:"sub_tasks"`
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
	Type        SubTaskType `json:"type"`
	Status      TaskStatus  `json:"status"`
	URL         string      `json:"url"`
	Description string      `json:"description"`
}

// SubTaskType represents the type of a subtask
type SubTaskType string

const (
	SubTaskTypeValidatingLink SubTaskType = "validating_link"
)

// AnalyzeResult represents the result of an analysis
type AnalyzeResult struct {
	HtmlVersion       string         `json:"html_version"`
	PageTitle         string         `json:"page_title"`
	Headings          map[string]int `json:"headings"`
	Links             []string       `json:"links"`
	InternalLinkCount int            `json:"internal_link_count"`
	ExternalLinkCount int            `json:"external_link_count"`
	AccessibleLinks   int            `json:"accessible_links"`
	InaccessibleLinks int            `json:"inaccessible_links"`
	HasLoginForm      bool           `json:"has_login_form"`
}
