package repository

import (
	"shared/models"
	"time"
)

// JobEntity represents a job as stored in DynamoDB
type JobEntity struct {
	PartitionKey string               `dynamodbav:"partition_key"`
	ID           string               `dynamodbav:"id"`
	URL          string               `dynamodbav:"url"`
	Status       string               `dynamodbav:"status"`
	CreatedAt    time.Time            `dynamodbav:"created_at"`
	UpdatedAt    time.Time            `dynamodbav:"updated_at"`
	StartedAt    *time.Time           `dynamodbav:"started_at"`
	CompletedAt  *time.Time           `dynamodbav:"completed_at"`
	Result       *AnalyzeResultEntity `dynamodbav:"result"`
}

// ToModel converts JobEntity to domain model
func (e *JobEntity) ToModel() *models.Job {
	var result *models.AnalyzeResult
	if e.Result != nil {
		result = e.Result.ToModel()
	}

	return &models.Job{
		ID:          e.ID,
		URL:         e.URL,
		Status:      models.JobStatus(e.Status),
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
		StartedAt:   e.StartedAt,
		CompletedAt: e.CompletedAt,
		Result:      result,
	}
}

// FromModel converts domain model to JobEntity
func (e *JobEntity) FromModel(job *models.Job) {
	e.PartitionKey = "1000" // Fixed partition key
	e.ID = job.ID
	e.URL = job.URL
	e.Status = string(job.Status)
	e.CreatedAt = job.CreatedAt
	e.UpdatedAt = job.UpdatedAt
	e.StartedAt = job.StartedAt
	e.CompletedAt = job.CompletedAt

	if job.Result != nil {
		e.Result = &AnalyzeResultEntity{}
		e.Result.FromModel(job.Result)
	}
}

// TaskEntity represents a task as stored in DynamoDB
type TaskEntity struct {
	JobID    string                   `dynamodbav:"job_id"`
	Type     string                   `dynamodbav:"type"`
	Status   string                   `dynamodbav:"status"`
	SubTasks map[string]SubTaskEntity `dynamodbav:"subtasks"`
}

// ToModel converts TaskEntity to domain model
func (e *TaskEntity) ToModel() *models.Task {
	subTasks := make(map[string]models.SubTask)
	for key, subTask := range e.SubTasks {
		subTasks[key] = *subTask.ToModel()
	}

	return &models.Task{
		JobID:    e.JobID,
		Type:     models.TaskType(e.Type),
		Status:   models.TaskStatus(e.Status),
		SubTasks: subTasks,
	}
}

// FromModel converts domain model to TaskEntity
func (e *TaskEntity) FromModel(task *models.Task) {
	e.JobID = task.JobID
	e.Type = string(task.Type)
	e.Status = string(task.Status)

	e.SubTasks = make(map[string]SubTaskEntity)
	for key, subTask := range task.SubTasks {
		entity := SubTaskEntity{}
		entity.FromModel(&subTask)
		e.SubTasks[key] = entity
	}
}

// AnalyzeResultEntity represents analysis result as stored in DynamoDB
type AnalyzeResultEntity struct {
	HtmlVersion       string         `dynamodbav:"html_version"`
	PageTitle         string         `dynamodbav:"page_title"`
	Headings          map[string]int `dynamodbav:"headings"`
	Links             []string       `dynamodbav:"links"`
	InternalLinkCount int            `dynamodbav:"internal_link_count"`
	ExternalLinkCount int            `dynamodbav:"external_link_count"`
	AccessibleLinks   int            `dynamodbav:"accessible_links"`
	InaccessibleLinks int            `dynamodbav:"inaccessible_links"`
	HasLoginForm      bool           `dynamodbav:"has_login_form"`
}

// ToModel converts AnalyzeResultEntity to domain model
func (e *AnalyzeResultEntity) ToModel() *models.AnalyzeResult {
	return &models.AnalyzeResult{
		HtmlVersion:       e.HtmlVersion,
		PageTitle:         e.PageTitle,
		Headings:          e.Headings,
		Links:             e.Links,
		InternalLinkCount: e.InternalLinkCount,
		ExternalLinkCount: e.ExternalLinkCount,
		AccessibleLinks:   e.AccessibleLinks,
		InaccessibleLinks: e.InaccessibleLinks,
		HasLoginForm:      e.HasLoginForm,
	}
}

// FromModel converts domain model to AnalyzeResultEntity
func (e *AnalyzeResultEntity) FromModel(result *models.AnalyzeResult) {
	e.HtmlVersion = result.HtmlVersion
	e.PageTitle = result.PageTitle
	e.Headings = result.Headings
	e.Links = result.Links
	e.InternalLinkCount = result.InternalLinkCount
	e.ExternalLinkCount = result.ExternalLinkCount
	e.AccessibleLinks = result.AccessibleLinks
	e.InaccessibleLinks = result.InaccessibleLinks
	e.HasLoginForm = result.HasLoginForm
}

// SubTaskEntity represents a subtask as stored in DynamoDB
type SubTaskEntity struct {
	Type        string `dynamodbav:"type"`
	Status      string `dynamodbav:"status"`
	URL         string `dynamodbav:"url"`
	Description string `dynamodbav:"description"`
}

// ToModel converts SubTaskEntity to domain model
func (e *SubTaskEntity) ToModel() *models.SubTask {
	return &models.SubTask{
		Type:        models.SubTaskType(e.Type),
		Status:      models.TaskStatus(e.Status),
		URL:         e.URL,
		Description: e.Description,
	}
}

// FromModel converts domain model to SubTaskEntity
func (e *SubTaskEntity) FromModel(subTask *models.SubTask) {
	e.Type = string(subTask.Type)
	e.Status = string(subTask.Status)
	e.URL = subTask.URL
	e.Description = subTask.Description
}
