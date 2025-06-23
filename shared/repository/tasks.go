package repository

import (
	"errors"
	"shared/types"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const TasksTableName = "web-analyzer-tasks"

type TaskRepository struct {
	dynamodb *dynamodb.DynamoDB
}

func NewTaskRepository() (*TaskRepository, error) {
	ddb, err := NewDynamoDBClient()
	if err != nil {
		return nil, err
	}

	return &TaskRepository{dynamodb: ddb}, nil
}

func (t *TaskRepository) CreateTasks(tasks ...*types.Task) error {
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

	_, err := t.dynamodb.BatchWriteItem(input)
	if err != nil {
		return err
	}

	return nil
}

func (t *TaskRepository) UpdateTaskStatus(jobId string, taskType types.TaskType, status types.TaskStatus) error {
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

	_, err := t.dynamodb.UpdateItem(input)
	return err
}

func (t *TaskRepository) AddSubTaskByKey(jobId string, taskType types.TaskType, key string, subtask types.SubTask) error {
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

	_, err = t.dynamodb.UpdateItem(input)
	return err
}

func (t *TaskRepository) UpdateSubTaskStatusByKey(jobId string, taskType types.TaskType, key string, status types.TaskStatus) error {
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
		UpdateExpression: aws.String("SET subtasks.#key.#status = :status"),
		ExpressionAttributeNames: map[string]*string{
			"#key":    aws.String(key),
			"#status": aws.String("status"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":status": {
				S: aws.String(string(status)),
			},
		},
		ConditionExpression: aws.String("attribute_exists(subtasks.#key)"),
	}

	_, err := t.dynamodb.UpdateItem(input)
	if err != nil {
		if strings.Contains(err.Error(), "ConditionalCheckFailedException") {
			return errors.New("subtask not found for the given key")
		}
		return err
	}

	return nil
}

func (t *TaskRepository) GetTasksByJobId(jobId string) ([]types.Task, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(TasksTableName),
		KeyConditionExpression: aws.String("job_id = :job_id"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":job_id": {
				S: aws.String(jobId),
			},
		},
	}

	result, err := t.dynamodb.Query(input)
	if err != nil {
		return nil, err
	}

	var tasks []types.Task
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
