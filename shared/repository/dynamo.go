package repository

import (
	"log"

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

func SeedTables(client *dynamodb.DynamoDB) error {
	jobsTableName := "web-analyzer-jobs"
	tasksTableName := "web-analyzer-tasks"

	err := createJobsTableIfNotExists(client, jobsTableName)
	if err != nil {
		return err
	}

	err = createTasksTableIfNotExists(client, tasksTableName)
	if err != nil {
		return err
	}

	log.Println("DynamoDB tables seeded successfully")
	return nil
}

func createJobsTableIfNotExists(client *dynamodb.DynamoDB, tableName string) error {
	// Check if table exists
	_, err := client.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		return nil // Table already exists
	}

	input := &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("id"),
				KeyType:       aws.String("HASH"),
			},
		},
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("id"),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String("status"),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String("created_at"),
				AttributeType: aws.String("S"),
			},
		},
		BillingMode: aws.String("PAY_PER_REQUEST"),
		GlobalSecondaryIndexes: []*dynamodb.GlobalSecondaryIndex{
			{
				IndexName: aws.String("status-created_at-index"),
				KeySchema: []*dynamodb.KeySchemaElement{
					{
						AttributeName: aws.String("status"),
						KeyType:       aws.String("HASH"),
					},
					{
						AttributeName: aws.String("created_at"),
						KeyType:       aws.String("RANGE"),
					},
				},
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String("ALL"),
				},
			},
		},
	}

	_, err = client.CreateTable(input)
	if err != nil {
		return err
	}

	log.Printf("Created DynamoDB jobs table: %s", tableName)
	return nil
}

func createTasksTableIfNotExists(client *dynamodb.DynamoDB, tableName string) error {
	// Check if table exists
	_, err := client.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		return nil // Table already exists
	}

	input := &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("id"),
				KeyType:       aws.String("HASH"),
			},
		},
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("id"),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String("job_id"),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String("status"),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String("created_at"),
				AttributeType: aws.String("S"),
			},
		},
		BillingMode: aws.String("PAY_PER_REQUEST"),
		GlobalSecondaryIndexes: []*dynamodb.GlobalSecondaryIndex{
			{
				IndexName: aws.String("job_id-created_at-index"),
				KeySchema: []*dynamodb.KeySchemaElement{
					{
						AttributeName: aws.String("job_id"),
						KeyType:       aws.String("HASH"),
					},
					{
						AttributeName: aws.String("created_at"),
						KeyType:       aws.String("RANGE"),
					},
				},
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String("ALL"),
				},
			},
			{
				IndexName: aws.String("status-created_at-index"),
				KeySchema: []*dynamodb.KeySchemaElement{
					{
						AttributeName: aws.String("status"),
						KeyType:       aws.String("HASH"),
					},
					{
						AttributeName: aws.String("created_at"),
						KeyType:       aws.String("RANGE"),
					},
				},
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String("ALL"),
				},
			},
		},
	}

	_, err = client.CreateTable(input)
	if err != nil {
		return err
	}

	log.Printf("Created DynamoDB tasks table: %s", tableName)
	return nil
}
