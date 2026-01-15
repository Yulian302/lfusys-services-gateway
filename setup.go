package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/Yulian302/lfusys-services-commons/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/sdk/trace"
)

type App struct {
	DynamoDB *dynamodb.Client
	Redis    *redis.Client

	Config    config.Config
	AwsConfig aws.Config

	Services       *Services
	TracerProvider *trace.TracerProvider
}

func SetupApp() (*App, error) {
	cfg := config.LoadConfig()

	if err := cfg.ValidateAllSecrets(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	awsCfg, err := initAWS(*cfg.AWSConfig)
	if err != nil {
		return nil, err
	}

	db := initDynamo(awsCfg)
	if db == nil {
		return nil, errors.New("could not init dynamodb")
	}

	rdb := initRedis(*cfg.RedisConfig)
	if rdb == nil {
		return nil, errors.New("could not init redis")
	}

	app := &App{
		DynamoDB: db,
		Redis:    rdb,

		Config:    cfg,
		AwsConfig: awsCfg,
	}

	app.Services = BuildServices(app)

	return app, nil
}

func (a *App) Run(r *gin.Engine) error {
	if err := r.Run(a.Config.GatewayAddr); err != nil {
		return err
	}
	return nil
}

func initAWS(cfg config.AWSConfig) (aws.Config, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.TODO(),
		awsconfig.WithRegion(cfg.Region),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load aws config: %w", err)
	}
	return awsCfg, nil
}

func initDynamo(cfg aws.Config) *dynamodb.Client {
	return dynamodb.NewFromConfig(cfg)
}

func initRedis(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.HOST,
		Password: "",
		DB:       0,
	})
}

func (a *App) Shutdown(ctx context.Context) {
	if a.Services != nil && a.Services.Conn != nil {
		_ = a.Services.Conn.Close()
	}
	if a.TracerProvider != nil {
		_ = a.TracerProvider.Shutdown(ctx)
	}
}
