package repository

import (
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

func NewDynamoDBClient() (*dynamodb.DynamoDB, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:   aws.String("us-east-1"),
		Endpoint: aws.String("http://localhost:8000"),
		Credentials: credentials.NewCredentials(&credentials.StaticProvider{
			Value: credentials.Value{
				AccessKeyID:     "DUMMYIDEXAMPLE",
				SecretAccessKey: "DUMMYIDEXAMPLE",
			},
		}),
	})
	if err != nil {
		return nil, err
	}

	client := dynamodb.New(sess)
	return client, nil
}

func SeedTables(client *dynamodb.DynamoDB, mc MetricsCollector) error {
	jobsTableName := "web-analyzer-jobs"
	tasksTableName := "web-analyzer-tasks"

	err := createJobsTableIfNotExists(client, jobsTableName, mc)
	if err != nil {
		return err
	}

	err = createTasksTableIfNotExists(client, tasksTableName, mc)
	if err != nil {
		return err
	}

	return nil
}

func createJobsTableIfNotExists(client *dynamodb.DynamoDB, tableName string, mc MetricsCollector) error {
	// Check if table exists
	_, err := client.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		return nil // Table already exists
	}

	start := time.Now()
	defer mc.RecordDatabaseOperation("create", tableName, start, nil)

	input := &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("partition_key"),
				KeyType:       aws.String("HASH"),
			},
			{
				AttributeName: aws.String("id"),
				KeyType:       aws.String("RANGE"),
			},
		},
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("partition_key"),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String("id"),
				AttributeType: aws.String("S"),
			},
		},
		BillingMode: aws.String("PAY_PER_REQUEST"),
	}

	_, err = client.CreateTable(input)
	if err != nil {
		return err
	}

	slog.Info("Created DynamoDB jobs table", "table", tableName)
	return nil
}

func createTasksTableIfNotExists(client *dynamodb.DynamoDB, tableName string, mc MetricsCollector) error {
	// Check if table exists
	_, err := client.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		return nil // Table already exists
	}

	start := time.Now()
	defer mc.RecordDatabaseOperation("create", tableName, start, nil)

	input := &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("job_id"),
				KeyType:       aws.String("HASH"),
			},
			{
				AttributeName: aws.String("type"),
				KeyType:       aws.String("RANGE"),
			},
		},
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("job_id"),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String("type"),
				AttributeType: aws.String("S"),
			},
		},
		BillingMode: aws.String("PAY_PER_REQUEST"),
	}

	_, err = client.CreateTable(input)
	if err != nil {
		return err
	}

	slog.Info("Created DynamoDB tasks table", "table", tableName)
	return nil
}
