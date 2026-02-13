package store

import (
	"context"
	"time"

	"github.com/Yulian302/lfusys-services-commons/health"
	"github.com/Yulian302/lfusys-services-commons/retries"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type UploadsStore interface {
	FindExisting(ctx context.Context, email string) (bool, error)

	health.ReadinessCheck
}

type DynamoDbUploadsStore struct {
	Client    *dynamodb.Client
	TableName string
}

func NewUploadsStore(dbClient *dynamodb.Client, tableName string) *DynamoDbUploadsStore {
	return &DynamoDbUploadsStore{
		Client:    dbClient,
		TableName: tableName,
	}
}

func (s *DynamoDbUploadsStore) IsReady(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	return retries.Retry(
		ctx,
		retries.HealthAttempts,
		retries.HealthBaseDelay,
		func() error {
			_, err := s.Client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
				TableName: aws.String(s.TableName),
			})

			return err
		},
		retries.IsRetriableDbError,
	)
}

func (s *DynamoDbUploadsStore) Name() string {
	return "UploadsStore[sessions]"
}

func (s *DynamoDbUploadsStore) FindExisting(ctx context.Context, email string) (bool, error) {
	var exists bool

	err := retries.Retry(
		ctx,
		retries.DefaultAttempts,
		retries.DefaultBaseDelay,
		func() error {
			out, err := s.Client.Query(ctx, &dynamodb.QueryInput{
				TableName:              &s.TableName,
				IndexName:              aws.String("user_email-index"),
				KeyConditionExpression: aws.String("user_email = :email"),
				ExpressionAttributeValues: map[string]dynamoTypes.AttributeValue{
					":email": &dynamoTypes.AttributeValueMemberS{Value: email},
				},
			})
			if err != nil {
				return err
			}
			if len(out.Items) > 0 {
				for _, item := range out.Items {
					if status, ok := item["status"]; ok {
						if statusStr := status.(*dynamoTypes.AttributeValueMemberS).Value; statusStr == "pending" || statusStr == "in_progress" {
							exists = true
							return nil
						}
					}
				}
			}

			return nil
		},
		retries.IsRetriableDbError,
	)

	return exists, err
}
