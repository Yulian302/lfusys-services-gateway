package store

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

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

func (store *DynamoDbStore) CreateUser(ctx context.Context) error {
	return nil
}
