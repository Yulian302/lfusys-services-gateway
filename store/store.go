package store

import (
	"context"
	"errors"

	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type UserStore interface {
	GetByEmail(ctx context.Context, email string) (*types.User, error)
	Create(ctx context.Context, user types.User) error
}

type UploadsStore interface {
	FindExisting(ctx context.Context, email string) (bool, error)
}

type DynamoDbStore struct {
	Client           *dynamodb.Client
	UsersTableName   string
	UploadsTableName string
}

func NewStore(dbClient *dynamodb.Client, utable string, uptable string) *DynamoDbStore {
	return &DynamoDbStore{
		Client:           dbClient,
		UsersTableName:   utable,
		UploadsTableName: uptable,
	}
}

func (s *DynamoDbStore) GetByEmail(ctx context.Context, email string) (*types.User, error) {
	res, err := s.Client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.UsersTableName),
		Key: map[string]dynamoTypes.AttributeValue{
			"email": &dynamoTypes.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil || res.Item == nil {
		return nil, errors.New("user not found")
	}

	var user types.User
	if err := attributevalue.UnmarshalMap(res.Item, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *DynamoDbStore) Create(ctx context.Context, user types.User) error {
	item, err := attributevalue.MarshalMap(user)
	if err != nil {
		return err
	}

	_, err = s.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.UsersTableName),
		Item:      item,
	})
	return err
}

func (s *DynamoDbStore) FindExisting(ctx context.Context, email string) (bool, error) {
	out, err := s.Client.Query(ctx, &dynamodb.QueryInput{
		TableName:              &s.UploadsTableName,
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
