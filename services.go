package main

import (
	"context"
	"log"
	"time"

	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/Yulian302/lfusys-services-commons/caching"
	"github.com/Yulian302/lfusys-services-gateway/auth/oauth"
	"github.com/Yulian302/lfusys-services-gateway/services"
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
}

type Providers struct {
	Github oauth.Provider
	Google oauth.Provider
}

type Services struct {
	Auth    services.AuthService
	Uploads services.UploadsService
	Files   services.FileService

	Stores *Stores

	Providers *Providers

	Conn *grpc.ClientConn
}

type Shutdowner interface {
	Shutdown(context.Context) error
}

func BuildServices(app *App) *Services {

	clientHandler := otelgrpc.NewClientHandler(
		otelgrpc.WithMessageEvents(otelgrpc.ReceivedEvents), // Record message events
	)
	conn, err := grpc.NewClient(app.Config.ServiceConfig.SessionGRPCUrl, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(clientHandler))
	if err != nil {
		panic(err)
	}

	usrStore := store.NewUserStore(app.DynamoDB, app.Config.DynamoDBConfig.UsersTableName)
	sessStore := store.NewRedisStoreImpl(app.Redis)
	upStore := store.NewUploadsStore(app.DynamoDB, app.Config.DynamoDBConfig.UploadsTableName)
	clientStub := pb.NewUploaderClient(conn)

	githubProvider := oauth.NewGithubProvider(app.Config.GithubConfig)
	googleProvider := oauth.NewGoogleProvider(app.Config.GoogleConfig)

	cacheSvc := caching.NewRedisCachingService(app.Redis)
	authSvc := services.NewAuthServiceImpl(usrStore, sessStore, cacheSvc, app.Config.JWTConfig.SecretKey, app.Config.JWTConfig.RefreshSecretKey)

	uploadsBreaker := gobreaker.NewCircuitBreaker[*pb.UploadReply](gobreaker.Settings{
		Name: "session-service:upload",

		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},

		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("circuit breaker %s: %s → %s", name, from, to)
		},
	})
	uploadsService := services.NewUploadsService(upStore, clientStub, uploadsBreaker)

	fileBreaker := gobreaker.NewCircuitBreaker[*pb.FilesReply](gobreaker.Settings{
		Name: "session-service:get-files",

		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},

		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("circuit breaker %s: %s → %s", name, from, to)
		},
	})
	fileService := services.NewFileServiceImpl(clientStub, fileBreaker)

	return &Services{
		Auth:    authSvc,
		Uploads: uploadsService,
		Files:   fileService,

		Stores: &Stores{
			users:    usrStore,
			sessions: sessStore,
			uploads:  upStore,
		},

		Providers: &Providers{
			Github: githubProvider,
			Google: googleProvider,
		},

		Conn: conn,
	}
}

func (s *Services) Shutdown(ctx context.Context) error {
	log.Println("shutting down services")

	if s.Stores != nil {
		if err := s.Stores.Shutdown(ctx); err != nil {
			log.Printf("stores shutdown error: %v", err)
		}
	}

	if s.Conn != nil {
		if err := s.Conn.Close(); err != nil {
			log.Printf("grpc conn close error: %v", err)
		}
	}

	log.Println("services shutdown complete")
	return nil
}

func (s *Stores) Shutdown(ctx context.Context) error {
	log.Println("shutting down stores")

	shutdownIfPossible := func(name string, v any) {
		if sh, ok := v.(Shutdowner); ok {
			if err := sh.Shutdown(ctx); err != nil {
				log.Printf("%s store shutdown error: %v", name, err)
			}
		}
	}

	shutdownIfPossible("users", s.users)
	shutdownIfPossible("sessions", s.sessions)
	shutdownIfPossible("uploads", s.uploads)

	log.Println("stores shutdown complete")
	return nil
}
