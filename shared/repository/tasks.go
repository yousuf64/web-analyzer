package repository

import (
	"context"
	"shared/config"
	"shared/models"
	"shared/tracing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const TasksTableName = "web-analyzer-tasks"

//go:generate mockgen -destination=../mocks/mock_tasks.go -package=mocks . TaskRepositoryInterface

type TaskRepositoryInterface interface {
	CreateTasks(ctx context.Context, tasks ...*models.Task) error
	UpdateTaskStatus(ctx context.Context, jobId string, taskType models.TaskType, status models.TaskStatus) error
	GetTasksByJobId(ctx context.Context, jobId string) ([]models.Task, error)
	AddSubTaskByKey(ctx context.Context, jobId string, taskType models.TaskType, key string, subtask models.SubTask) error
	UpdateSubTaskByKey(ctx context.Context, jobId string, taskType models.TaskType, key string, subtask models.SubTask) error
}

// TaskOption is a function that configures the TaskRepository
type TaskOption func(*TaskRepository)

// WithTaskMetrics sets the metrics collector
func WithTaskMetrics(mc MetricsCollector) TaskOption {
	return func(t *TaskRepository) {
		t.mc = mc
	}
}

// TaskRepository is a struct for task repository
type TaskRepository struct {
	ddb *dynamodb.DynamoDB
	mc  MetricsCollector
}

// NewTaskRepository creates a new task repository
func NewTaskRepository(cfg config.DynamoDBConfig, opts ...TaskOption) (*TaskRepository, error) {
	ddb, err := NewDynamoDBClient(cfg)
	if err != nil {
		return nil, err
	}

	repo := &TaskRepository{ddb: ddb, mc: NoOpMetricsCollector{}}
	for _, opt := range opts {
		opt(repo)
	}

	return repo, nil
}

// CreateTasks creates tasks
func (t *TaskRepository) CreateTasks(ctx context.Context, tasks ...*models.Task) (err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "create_tasks", TasksTableName)

	defer func() {
		t.mc.RecordDatabaseOperation("create_tasks", TasksTableName, start, err)
		span.Close(err)
	}()

	writeRequests := make([]*dynamodb.WriteRequest, 0, len(tasks))

	for _, task := range tasks {
		// Convert domain model to entity
		entity := &TaskEntity{}
		entity.FromModel(task)

		item, err := dynamodbattribute.MarshalMap(entity)
		if err != nil {
			return err
		}

		if len(task.SubTasks) == 0 {
			// Initialize subtasks as empty map
			item["subtasks"] = &dynamodb.AttributeValue{
				M: map[string]*dynamodb.AttributeValue{},
			}
		}

		writeRequests = append(writeRequests, &dynamodb.WriteRequest{
			PutRequest: &dynamodb.PutRequest{
				Item: item,
			},
		})
	}

	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]*dynamodb.WriteRequest{
			TasksTableName: writeRequests,
		},
	}

	_, err = t.ddb.BatchWriteItem(input)
	return err
}

// UpdateTaskStatus updates task status
func (t *TaskRepository) UpdateTaskStatus(ctx context.Context, jobId string, taskType models.TaskType, status models.TaskStatus) (err error) {
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

// GetTasksByJobId queries tasks by job ID
func (t *TaskRepository) GetTasksByJobId(ctx context.Context, jobId string) (tasks []models.Task, err error) {
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

	tasks = make([]models.Task, 0, len(result.Items))
	for _, item := range result.Items {
		var entity TaskEntity
		err = dynamodbattribute.UnmarshalMap(item, &entity)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *entity.ToModel())
	}

	return tasks, nil
}

// AddSubTaskByKey adds a subtask by key
func (t *TaskRepository) AddSubTaskByKey(ctx context.Context, jobId string, taskType models.TaskType, key string, subtask models.SubTask) (err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "add_subtask", TasksTableName)

	defer func() {
		t.mc.RecordDatabaseOperation("add_subtask", TasksTableName, start, err)
		span.Close(err)
	}()

	// Convert domain model to entity
	entity := &SubTaskEntity{}
	entity.FromModel(&subtask)

	subtaskMap, err := dynamodbattribute.MarshalMap(entity)
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
				M: subtaskMap,
			},
		},
	}

	_, err = t.ddb.UpdateItem(input)
	return err
}

// UpdateSubTaskByKey updates a subtask by key
func (t *TaskRepository) UpdateSubTaskByKey(ctx context.Context, jobId string, taskType models.TaskType, key string, subtask models.SubTask) (err error) {
	start := time.Now()
	_, span := tracing.CreateDatabaseSpan(ctx, "update_subtask", TasksTableName)

	defer func() {
		t.mc.RecordDatabaseOperation("update_subtask", TasksTableName, start, err)
		span.Close(err)
	}()

	// Convert domain model to entity
	entity := &SubTaskEntity{}
	entity.FromModel(&subtask)

	subtaskMap, err := dynamodbattribute.MarshalMap(entity)
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
				M: subtaskMap,
			},
		},
	}

	_, err = t.ddb.UpdateItem(input)
	return err
}
