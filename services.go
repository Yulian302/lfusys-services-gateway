package main

import (
	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/Yulian302/lfusys-services-commons/caching"
	"github.com/Yulian302/lfusys-services-gateway/auth/oauth"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/Yulian302/lfusys-services-gateway/store"
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
	uploadsService := services.NewUploadsService(upStore, clientStub)
	fileService := services.NewFileServiceImpl(clientStub)

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
