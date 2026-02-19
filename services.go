package main

import (
	"context"
	"fmt"
	"time"

	pb "github.com/Yulian302/lfusys-services-commons/api/uploader/v1"
	"github.com/Yulian302/lfusys-services-commons/caching"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-gateway/auth/oauth"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/Yulian302/lfusys-services-gateway/services/auth"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Stores struct {
	users    store.UserStore
	sessions store.SessionStore
	uploads  store.UploadsStore

	logger logger.Logger
}

type Providers struct {
	Github oauth.Provider
	Google oauth.Provider
}

type Services struct {
	JwtAuth      auth.JwtAuthService
	OAuth        auth.OAuthService
	StateManager auth.StateManager
	Uploads      services.UploadsService
	Files        services.FileService

	Stores *Stores

	Providers *Providers

	Conn   *grpc.ClientConn
	logger logger.Logger
}

type Shutdowner interface {
	Shutdown(context.Context) error
}

func BuildServices(app *App) (*Services, error) {
	conn, err := grpc.NewClient(
		app.Config.ServiceConfig.SessionGRPCUrl,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		app.Logger.Error("failed to create gRPC client",
			"url", app.Config.ServiceConfig.SessionGRPCUrl,
			"error", err,
		)
		return nil, fmt.Errorf("create grpc client: %w", err)
	}

	app.Logger.Info("gRPC client created successfully",
		"url", app.Config.ServiceConfig.SessionGRPCUrl,
	)

	usrStore := store.NewUserStore(app.DynamoDB, app.Config.DynamoDBConfig.UsersTableName)
	sessStore := store.NewRedisStoreImpl(app.Redis)
	upStore := store.NewUploadsStore(app.DynamoDB, app.Config.DynamoDBConfig.UploadsTableName)
	clientStub := pb.NewUploaderClient(conn)

	githubProvider := oauth.NewGithubProvider(app.Config.GithubConfig)
	googleProvider := oauth.NewGoogleProvider(app.Config.GoogleConfig)

	cacheSvc := caching.NewRedisCachingService(app.Redis, app.Logger)
	jwtAuthSvc := auth.NewJwtAuthServiceImpl(
		auth.JwtAuthServiceDeps{
			UserStore:     usrStore,
			Cache:         cacheSvc,
			AccessSecret:  app.Config.JWTConfig.SecretKey,
			RefreshSecret: app.Config.JWTConfig.RefreshSecretKey,
			Logger:        app.Logger,
		},
	)
	oAuthSvc := auth.NewOAuthServiceImpl(auth.OAuthServiceDeps{
		UserStore:     usrStore,
		Cache:         cacheSvc,
		AccessSecret:  app.Config.JWTConfig.SecretKey,
		RefreshSecret: app.Config.JWTConfig.RefreshSecretKey,
		Logger:        app.Logger,
	})
	regStateManager := auth.NewRegisterStateManager(sessStore, app.Logger)

	uploadsBreaker := gobreaker.NewCircuitBreaker[*pb.UploadReply](gobreaker.Settings{
		Name: "session-service:upload",

		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},

		OnStateChange: func(name string, from, to gobreaker.State) {
			app.Logger.Info(fmt.Sprintf("circuit breaker %s: %s → %s", name, from, to))
		},
	})

	var chunkSize int64
	if app.Config.Env == "DEV" {
		chunkSize = 128 * 1024
	} else {
		chunkSize = 5 * 1024 * 1024
	}
	uploadsService := services.NewUploadsService(services.UploadsServiceDeps{
		Store:     upStore,
		Client:    clientStub,
		Breaker:   uploadsBreaker,
		ChunkSize: chunkSize,
		Logger:    app.Logger,
	})

	fileBreaker := gobreaker.NewCircuitBreaker[any](gobreaker.Settings{
		Name: "session-service:get-files",

		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},

		OnStateChange: func(name string, from, to gobreaker.State) {
			app.Logger.Info(fmt.Sprintf("circuit breaker %s: %s → %s", name, from, to))
		},
	})
	fileService := services.NewFileServiceImpl(clientStub, fileBreaker, app.Logger)

	app.Logger.Info("gateway services initialized successfully")

	return &Services{
		JwtAuth:      jwtAuthSvc,
		OAuth:        oAuthSvc,
		StateManager: regStateManager,
		Uploads:      uploadsService,
		Files:        fileService,

		Stores: &Stores{
			users:    usrStore,
			sessions: sessStore,
			uploads:  upStore,
		},

		Providers: &Providers{
			Github: githubProvider,
			Google: googleProvider,
		},

		Conn:   conn,
		logger: app.Logger,
	}, nil
}

func (s *Services) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down services")

	if s.Stores != nil {
		if err := s.Stores.Shutdown(ctx); err != nil {
			s.logger.Error("stores shutdown failed", "err", err.Error())
		}
	}

	if s.Conn != nil {
		if err := s.Conn.Close(); err != nil {
			s.logger.Error("gRPC connection close failed", "err", err.Error())
		}
	}

	s.logger.Info("services shutdown complete")
	return nil
}

func (s *Stores) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down stores")

	shutdownIfPossible := func(name string, v any) {
		if sh, ok := v.(Shutdowner); ok {
			if err := sh.Shutdown(ctx); err != nil {
				s.logger.Error(fmt.Sprintf("%s store shutdown failed", name), "err", err.Error())
			}
		}
	}

	shutdownIfPossible("users", s.users)
	shutdownIfPossible("sessions", s.sessions)
	shutdownIfPossible("uploads", s.uploads)

	s.logger.Info("stores shutdown complete")
	return nil
}
