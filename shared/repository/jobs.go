package repository

import (
	"errors"
	"shared/types"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const JobsTableName = "web-analyzer-jobs"

type JobRepository struct {
	dynamodb *dynamodb.DynamoDB
}

func NewJobRepository() (*JobRepository, error) {
	ddb, err := NewDynamoDBClient()
	if err != nil {
		return nil, err
	}

	return &JobRepository{dynamodb: ddb}, nil
}

func (j *JobRepository) CreateJob(job *types.Job) error {
	item, err := dynamodbattribute.MarshalMap(job)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(JobsTableName),
		Item:      item,
	}

	_, err = j.dynamodb.PutItem(input)
	return err
}

func (j *JobRepository) GetJob(id string) (*types.Job, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(JobsTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(id),
			},
		},
	}

	result, err := j.dynamodb.GetItem(input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, errors.New("job not found")
	}

	var job types.Job
	err = dynamodbattribute.UnmarshalMap(result.Item, &job)
	if err != nil {
		return nil, err
	}

	return &job, nil
}

func (j *JobRepository) UpdateJobStatus(id string, status types.JobStatus) error {
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(JobsTableName),
		Key: map[string]*dynamodb.AttributeValue{
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

	_, err := j.dynamodb.UpdateItem(input)
	return err
}

func (j *JobRepository) UpdateJob(id string, status *types.JobStatus, result *types.AnalyzeResult) error {
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
		expressionAttributeValues[":result"] = resultAttr
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(JobsTableName),
		Key: map[string]*dynamodb.AttributeValue{
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

	_, err := j.dynamodb.UpdateItem(input)
	return err
}
