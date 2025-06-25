package repository

import (
	"context"
	"errors"
	"shared/tracing"
	"shared/types"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const JobsTableName = "web-analyzer-jobs"

type JobRepository struct {
	ddb *dynamodb.DynamoDB
	mc  MetricsCollector
}

// NewJobRepository creates a new JobRepository with the given metrics collector
func NewJobRepository(mc MetricsCollector) (*JobRepository, error) {
	ddb, err := NewDynamoDBClient()
	if err != nil {
		return nil, err
	}

	if mc == nil {
		mc = NoOpMetricsCollector{}
	}

	return &JobRepository{
		ddb: ddb,
		mc:  mc,
	}, nil
}

func (j *JobRepository) CreateJob(ctx context.Context, job *types.Job) (err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "create_job", JobsTableName)

	defer func() {
		j.mc.RecordDatabaseOperation("create_job", JobsTableName, start, err)
		span.Close(err)
	}()

	job.PartitionKey = "1000"
	item, err := dynamodbattribute.MarshalMap(job)
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

func (j *JobRepository) GetJob(ctx context.Context, id string) (job *types.Job, err error) {
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

	err = dynamodbattribute.UnmarshalMap(result.Item, &job)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (j *JobRepository) GetAllJobs(ctx context.Context) (jobs []*types.Job, err error) {
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

	jobs = make([]*types.Job, 0, len(result.Items))
	for _, item := range result.Items {
		var job types.Job
		err = dynamodbattribute.UnmarshalMap(item, &job)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

func (j *JobRepository) UpdateJobStatus(ctx context.Context, id string, status types.JobStatus) (err error) {
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

func (j *JobRepository) UpdateJob(ctx context.Context, id string, status *types.JobStatus, result *types.AnalyzeResult) (err error) {
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
		resultAttr, err := dynamodbattribute.Marshal(result)
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
	if len(expressionAttributeValues) > 0 {
		input.ExpressionAttributeValues = expressionAttributeValues
	}

	_, err = j.ddb.UpdateItem(input)
	return err
}
