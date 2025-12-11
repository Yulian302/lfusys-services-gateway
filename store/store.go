package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// handles user auth
type DynamoDbStore struct {
	Client    *dynamodb.Client
	TableName string
}

func NewStore(dbClient *dynamodb.Client, tbname string) *DynamoDbStore {
	return &DynamoDbStore{
		Client:    dbClient,
		TableName: tbname,
	}
}

func (store *DynamoDbStore) CreateUser(ctx context.Context) error {
	return nil
}
