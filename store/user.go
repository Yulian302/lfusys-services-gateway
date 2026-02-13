package store

import (
	"context"
	"errors"
	"time"

	apperror "github.com/Yulian302/lfusys-services-commons/errors"
	"github.com/Yulian302/lfusys-services-commons/health"
	"github.com/Yulian302/lfusys-services-commons/retries"
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

func (s *DynamoDbUserStore) Name() string {
	return "UserStore[users]"
}

func (s *DynamoDbUserStore) GetByEmail(ctx context.Context, email string) (*types.User, error) {
	var user types.User

	err := retries.Retry(
		ctx,
		retries.DefaultAttempts,
		retries.DefaultBaseDelay,
		func() error {
			res, err := s.Client.GetItem(ctx, &dynamodb.GetItemInput{
				TableName: aws.String(s.TableName),
				Key: map[string]dynamoTypes.AttributeValue{
					"email": &dynamoTypes.AttributeValueMemberS{Value: email},
				},
			})
			if err != nil {
				return err
			}

			if res.Item == nil {
				return apperror.ErrUserNotFound
			}

			return attributevalue.UnmarshalMap(res.Item, &user)
		},
		retries.IsRetriableDbError,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *DynamoDbUserStore) Create(ctx context.Context, user types.User) error {
	item, err := attributevalue.MarshalMap(user)
	if err != nil {
		return err
	}

	return retries.Retry(
		ctx,
		retries.DefaultAttempts,
		retries.DefaultBaseDelay,
		func() error {
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
		},
		retries.IsRetriableDbError,
	)
}
