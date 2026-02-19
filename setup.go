package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	common "github.com/Yulian302/lfusys-services-commons"
	"github.com/Yulian302/lfusys-services-commons/config"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/sdk/trace"
)

type App struct {
	Server *http.Server

	DynamoDB *dynamodb.Client
	Redis    *redis.Client

	Config    config.Config
	AwsConfig aws.Config

	Services       *Services
	TracerProvider *trace.TracerProvider
	Logger         logger.Logger
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

	appLogger := logger.NewSlogLogger(logger.CreateAppLogger(cfg.Env))

	app := &App{
		DynamoDB: db,
		Redis:    rdb,

		Config:    cfg,
		AwsConfig: awsCfg,
		Logger:    appLogger,
	}

	if cfg.Tracing {
		tp, err := common.InitTracer(context.Background(), "gateway", cfg.TracingAddr)
		if err != nil {
			app.Logger.Error("tracing failed", "err", err.Error())
		}
		app.Logger.Info("tracing in progress...")

		app.TracerProvider = tp
	}

	app.Services = BuildServices(app)

	return app, nil
}

func (a *App) Run(r *gin.Engine) error {
	a.Server = &http.Server{
		Addr:         a.Config.GatewayAddr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return a.Server.ListenAndServe()
}

func initAWS(cfg config.AWSConfig) (aws.Config, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
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
		Addr:         cfg.HOST,
		Password:     "",
		DB:           0,
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
	})
}

func (a *App) Shutdown(ctx context.Context) error {
	a.Logger.Info("starting graceful shutdown")

	if a.Server != nil {
		if err := a.Server.Shutdown(ctx); err != nil {
			a.Logger.Error("http server shutdown failed", "err", err.Error())
		}
	}

	if a.Services != nil {
		if err := a.Services.Shutdown(ctx); err != nil {
			a.Logger.Error("services shutdown failed", "err", err.Error())
		}
	}

	if a.Redis != nil {
		if err := a.Redis.Close(); err != nil {
			a.Logger.Error("redis close failed", "err", err.Error())
		}
	}

	if a.TracerProvider != nil {
		if err := a.TracerProvider.Shutdown(ctx); err != nil {
			a.Logger.Error("tracer shutdown failed", "err", err.Error())
		}
	}

	a.Logger.Info("graceful shutdown complete")
	return nil
}
