package repository

import (
	"log/slog"
	"shared/config"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// NewDynamoDBClient creates a new DynamoDB client
func NewDynamoDBClient(cfg config.DynamoDBConfig) (*dynamodb.DynamoDB, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:   aws.String(cfg.Region),
		Endpoint: aws.String(cfg.Endpoint),
		Credentials: credentials.NewCredentials(&credentials.StaticProvider{
			Value: credentials.Value{
				AccessKeyID:     cfg.AccessKeyID,
				SecretAccessKey: cfg.SecretAccessKey,
			},
		}),
	})
	if err != nil {
		return nil, err
	}

	client := dynamodb.New(sess)
	return client, nil
}

// SeedTables seeds the DynamoDB tables
func SeedTables(client *dynamodb.DynamoDB, cfg config.DynamoDBConfig, mc MetricsCollector) error {
	err := createJobsTableIfNotExists(client, JobsTableName, mc)
	if err != nil {
		return err
	}

	err = createTasksTableIfNotExists(client, TasksTableName, mc)
	if err != nil {
		return err
	}

	return nil
}

// createJobsTableIfNotExists creates the jobs table if it doesn't exist
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
		if strings.Contains(err.Error(), "Cannot create preexisting table") {
			return nil
		}
		return err
	}

	slog.Info("Created DynamoDB jobs table", "table", tableName)
	return nil
}

// createTasksTableIfNotExists creates the tasks table if it doesn't exist
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
		if strings.Contains(err.Error(), "Cannot create preexisting table") {
			return nil
		}
		return err
	}

	slog.Info("Created DynamoDB tasks table", "table", tableName)
	return nil
}
