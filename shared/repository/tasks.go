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

const TasksTableName = "web-analyzer-tasks"

type TaskRepository struct {
	ddb *dynamodb.DynamoDB
	mc  MetricsCollector
}

// NewTaskRepository creates a new TaskRepository with the given metrics collector
func NewTaskRepository(mc MetricsCollector) (*TaskRepository, error) {
	ddb, err := NewDynamoDBClient()
	if err != nil {
		return nil, err
	}

	if mc == nil {
		mc = NoOpMetricsCollector{}
	}

	return &TaskRepository{
		ddb: ddb,
		mc:  mc,
	}, nil
}

func (t *TaskRepository) CreateTasks(ctx context.Context, tasks ...*types.Task) (err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "create_tasks", TasksTableName)

	defer func() {
		t.mc.RecordDatabaseOperation("create_tasks", TasksTableName, start, err)
		span.Close(err)
	}()

	if len(tasks) == 0 {
		return nil
	}

	var writeRequests []*dynamodb.WriteRequest
	for _, task := range tasks {
		item, err := dynamodbattribute.MarshalMap(task)
		if err != nil {
			return err
		}

		if len(task.SubTasks) == 0 {
			// Initialize subtasks as empty map
			item["subtasks"] = &dynamodb.AttributeValue{
				M: map[string]*dynamodb.AttributeValue{},
			}
		}

		writeRequest := &dynamodb.WriteRequest{
			PutRequest: &dynamodb.PutRequest{
				Item: item,
			},
		}
		writeRequests = append(writeRequests, writeRequest)
	}

	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]*dynamodb.WriteRequest{
			TasksTableName: writeRequests,
		},
	}

	_, err = t.ddb.BatchWriteItem(input)
	if err != nil {
		return err
	}

	return nil
}

func (t *TaskRepository) UpdateTaskStatus(ctx context.Context, jobId string, taskType types.TaskType, status types.TaskStatus) (err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "update_task_status", TasksTableName)

	defer func() {
		t.mc.RecordDatabaseOperation("update_task_status", TasksTableName, start, err)
		span.Close(err)
	}()

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(TasksTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"job_id": {
				S: aws.String(jobId),
			},
			"type": {
				S: aws.String(string(taskType)),
			},
		},
		UpdateExpression: aws.String("SET #status = :status"),
		ExpressionAttributeNames: map[string]*string{
			"#status": aws.String("status"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":status": {
				S: aws.String(string(status)),
			},
		},
	}

	_, err = t.ddb.UpdateItem(input)
	return err
}

func (t *TaskRepository) AddSubTaskByKey(ctx context.Context, jobId string, taskType types.TaskType, key string, subtask types.SubTask) (err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "add_subtask", TasksTableName)

	defer func() {
		t.mc.RecordDatabaseOperation("add_subtask", TasksTableName, start, err)
		span.Close(err)
	}()

	subtaskItem, err := dynamodbattribute.MarshalMap(subtask)
	if err != nil {
		return err
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(TasksTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"job_id": {
				S: aws.String(jobId),
			},
			"type": {
				S: aws.String(string(taskType)),
			},
		},
		UpdateExpression: aws.String("SET #subtasks.#key = :subtask"),
		ExpressionAttributeNames: map[string]*string{
			"#subtasks": aws.String("subtasks"),
			"#key":      aws.String(key),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":subtask": {
				M: subtaskItem,
			},
		},
	}

	_, err = t.ddb.UpdateItem(input)
	return err
}

func (t *TaskRepository) UpdateSubTaskByKey(ctx context.Context, jobId string, taskType types.TaskType, key string, subtask types.SubTask) (err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "update_subtask", TasksTableName)

	defer func() {
		t.mc.RecordDatabaseOperation("update_subtask", TasksTableName, start, err)
		span.Close(err)
	}()

	subtaskItem, err := dynamodbattribute.MarshalMap(subtask)
	if err != nil {
		return err
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(TasksTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"job_id": {
				S: aws.String(jobId),
			},
			"type": {
				S: aws.String(string(taskType)),
			},
		},
		UpdateExpression: aws.String("SET subtasks.#key = :subtask"),
		ExpressionAttributeNames: map[string]*string{
			"#key": aws.String(key),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":subtask": {
				M: subtaskItem,
			},
		},
		ConditionExpression: aws.String("attribute_exists(subtasks.#key)"),
	}

	_, err = t.ddb.UpdateItem(input)
	if err != nil {
		if strings.Contains(err.Error(), "ConditionalCheckFailedException") {
			conditionalErr := errors.New("subtask not found for the given key")
			return conditionalErr
		}
		return err
	}

	return nil
}

func (t *TaskRepository) GetTasksByJobId(ctx context.Context, jobId string) (tasks []types.Task, err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "query_tasks_by_job_id", TasksTableName)

	defer func() {
		t.mc.RecordDatabaseOperation("query_tasks_by_job_id", TasksTableName, start, err)
		span.Close(err)
	}()

	input := &dynamodb.QueryInput{
		TableName:              aws.String(TasksTableName),
		KeyConditionExpression: aws.String("job_id = :job_id"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":job_id": {
				S: aws.String(jobId),
			},
		},
	}

	result, err := t.ddb.Query(input)
	if err != nil {
		return nil, err
	}

	for _, item := range result.Items {
		var task types.Task
		err = dynamodbattribute.UnmarshalMap(item, &task)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}
