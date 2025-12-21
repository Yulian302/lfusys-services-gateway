package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type UploadsStore interface {
	FindExisting(ctx context.Context, email string) (bool, error)
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

func (s *DynamoDbUploadsStore) FindExisting(ctx context.Context, email string) (bool, error) {
	out, err := s.Client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &s.TableName,
		IndexName:              aws.String("user_email-index"),
		KeyConditionExpression: aws.String("user_email = :email"),
		ExpressionAttributeValues: map[string]dynamoTypes.AttributeValue{
			":email": &dynamoTypes.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		return false, err
	}
	if len(out.Items) > 0 {
		for _, item := range out.Items {
			if status, exists := item["status"]; exists {
				if statusStr := status.(*dynamoTypes.AttributeValueMemberS).Value; statusStr == "pending" || statusStr == "in_progress" {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
