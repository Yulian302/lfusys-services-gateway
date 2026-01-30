package store

import (
	"context"
	"errors"
	"time"

	apperror "github.com/Yulian302/lfusys-services-commons/errors"
	"github.com/Yulian302/lfusys-services-commons/health"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamoTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type UserStore interface {
	GetByEmail(ctx context.Context, email string) (*types.User, error)
	Create(ctx context.Context, user types.User) error

	health.ReadinessCheck
}

type DynamoDbUserStore struct {
	Client    *dynamodb.Client
	TableName string
}

func NewUserStore(dbClient *dynamodb.Client, tableName string) *DynamoDbUserStore {
	return &DynamoDbUserStore{
		Client:    dbClient,
		TableName: tableName,
	}
}

func (s *DynamoDbUserStore) IsReady(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	_, err := s.Client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(s.TableName),
	})

	return err
}

func (s *DynamoDbUserStore) Name() string {
	return "UserStore[users]"
}

func (s *DynamoDbUserStore) GetByEmail(ctx context.Context, email string) (*types.User, error) {
	res, err := s.Client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.TableName),
		Key: map[string]dynamoTypes.AttributeValue{
			"email": &dynamoTypes.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil || res.Item == nil {
		return nil, apperror.ErrUserNotFound
	}

	var user types.User
	if err := attributevalue.UnmarshalMap(res.Item, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *DynamoDbUserStore) Create(ctx context.Context, user types.User) error {
	item, err := attributevalue.MarshalMap(user)
	if err != nil {
		return err
	}

	_, err = s.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.TableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(email)"),
	})
	if err != nil {
		var ccf *dynamoTypes.ConditionalCheckFailedException
		if errors.As(err, &ccf) {
			return apperror.ErrUserAlreadyExists
		}
		return err
	}
	return nil
}
