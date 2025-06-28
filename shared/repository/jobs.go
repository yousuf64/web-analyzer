package repository

import (
	"context"
	"errors"
	"shared/config"
	"shared/models"
	"shared/tracing"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const JobsTableName = "web-analyzer-jobs"

// JobOption is a function that configures the JobRepository
type JobOption func(*JobRepository)

// WithJobMetrics sets the metrics collector
func WithJobMetrics(mc MetricsCollector) JobOption {
	return func(j *JobRepository) {
		j.mc = mc
	}
}

// JobRepository is a struct for job repository
type JobRepository struct {
	ddb *dynamodb.DynamoDB
	mc  MetricsCollector
}

// NewJobRepository creates a new job repository
func NewJobRepository(cfg config.DynamoDBConfig, opts ...JobOption) (*JobRepository, error) {
	ddb, err := NewDynamoDBClient(cfg)
	if err != nil {
		return nil, err
	}

	repo := &JobRepository{ddb: ddb, mc: NoOpMetricsCollector{}}
	for _, opt := range opts {
		opt(repo)
	}

	return repo, nil
}

// CreateJob creates a new job
func (j *JobRepository) CreateJob(ctx context.Context, job *models.Job) (err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "create_job", JobsTableName)

	defer func() {
		j.mc.RecordDatabaseOperation("create_job", JobsTableName, start, err)
		span.Close(err)
	}()

	// Convert domain model to entity
	entity := &JobEntity{}
	entity.FromModel(job)

	item, err := dynamodbattribute.MarshalMap(entity)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(JobsTableName),
		Item:      item,
	}

	_, err = j.ddb.PutItem(input)
	return err
}

// GetJob queries a job by ID
func (j *JobRepository) GetJob(ctx context.Context, id string) (job *models.Job, err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "get_job", JobsTableName)

	defer func() {
		j.mc.RecordDatabaseOperation("get_job", JobsTableName, start, err)
		span.Close(err)
	}()

	input := &dynamodb.GetItemInput{
		TableName: aws.String(JobsTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"partition_key": {
				S: aws.String("1000"),
			},
			"id": {
				S: aws.String(id),
			},
		},
	}

	result, err := j.ddb.GetItem(input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		notFoundErr := errors.New("job not found")
		return nil, notFoundErr
	}

	var entity JobEntity
	err = dynamodbattribute.UnmarshalMap(result.Item, &entity)
	if err != nil {
		return nil, err
	}

	return entity.ToModel(), nil
}

// GetAllJobs queries all jobs
func (j *JobRepository) GetAllJobs(ctx context.Context) (jobs []*models.Job, err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "query_all_jobs", JobsTableName)

	defer func() {
		j.mc.RecordDatabaseOperation("query_all_jobs", JobsTableName, start, err)
		span.Close(err)
	}()

	input := &dynamodb.QueryInput{
		TableName:              aws.String(JobsTableName),
		KeyConditionExpression: aws.String("#partition_key = :partition_key"),
		ExpressionAttributeNames: map[string]*string{
			"#partition_key": aws.String("partition_key"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":partition_key": {
				S: aws.String("1000"),
			},
		},
		ScanIndexForward: aws.Bool(false), // false for descending order since JobID is based on timestamp
	}

	result, err := j.ddb.Query(input)
	if err != nil {
		return nil, err
	}

	jobs = make([]*models.Job, 0, len(result.Items))
	for _, item := range result.Items {
		var entity JobEntity
		err = dynamodbattribute.UnmarshalMap(item, &entity)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, entity.ToModel())
	}

	return jobs, nil
}

// UpdateJobStatus updates the status of a job
func (j *JobRepository) UpdateJobStatus(ctx context.Context, id string, status models.JobStatus) (err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "update_job_status", JobsTableName)

	defer func() {
		j.mc.RecordDatabaseOperation("update_job_status", JobsTableName, start, err)
		span.Close(err)
	}()

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(JobsTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"partition_key": {
				S: aws.String("1000"),
			},
			"id": {
				S: aws.String(id),
			},
		},
		UpdateExpression: aws.String("SET #status = :status, updated_at = :updated_at"),
		ExpressionAttributeNames: map[string]*string{
			"#status": aws.String("status"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":status": {
				S: aws.String(string(status)),
			},
			":updated_at": {
				S: aws.String(time.Now().Format(time.RFC3339)),
			},
		},
	}

	_, err = j.ddb.UpdateItem(input)
	return err
}

// UpdateJob updates a job
func (j *JobRepository) UpdateJob(ctx context.Context, id string, status *models.JobStatus, result *models.AnalyzeResult) (err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "update_job", JobsTableName)

	defer func() {
		j.mc.RecordDatabaseOperation("update_job", JobsTableName, start, err)
		span.Close(err)
	}()

	var updateExpressions []string
	expressionAttributeValues := make(map[string]*dynamodb.AttributeValue)
	expressionAttributeNames := make(map[string]*string)

	updateExpressions = append(updateExpressions, "updated_at = :updated_at")
	expressionAttributeValues[":updated_at"] = &dynamodb.AttributeValue{
		S: aws.String(time.Now().Format(time.RFC3339)),
	}

	if status != nil {
		updateExpressions = append(updateExpressions, "#status = :status")
		expressionAttributeNames["#status"] = aws.String("status")
		expressionAttributeValues[":status"] = &dynamodb.AttributeValue{
			S: aws.String(string(*status)),
		}
	}

	if result != nil {
		updateExpressions = append(updateExpressions, "#result = :result")
		expressionAttributeNames["#result"] = aws.String("result")

		// Convert models.AnalyzeResult to AnalyzeResultEntity
		resultEntity := &AnalyzeResultEntity{}
		resultEntity.FromModel(result)

		resultAttr, err := dynamodbattribute.Marshal(resultEntity)
		if err != nil {
			return err
		}
		if len(result.Headings) == 0 {
			resultAttr.M["headings"] = &dynamodb.AttributeValue{
				M: make(map[string]*dynamodb.AttributeValue),
			}
		}

		if len(result.Links) == 0 {
			resultAttr.M["links"] = &dynamodb.AttributeValue{
				L: []*dynamodb.AttributeValue{},
			}
		}
		expressionAttributeValues[":result"] = resultAttr
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(JobsTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"partition_key": {
				S: aws.String("1000"),
			},
			"id": {
				S: aws.String(id),
			},
		},
		UpdateExpression:          aws.String("SET " + strings.Join(updateExpressions, ", ")),
		ExpressionAttributeValues: expressionAttributeValues,
	}

	if len(expressionAttributeNames) > 0 {
		input.ExpressionAttributeNames = expressionAttributeNames
	}

	_, err = j.ddb.UpdateItem(input)
	return err
}
