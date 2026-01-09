package main

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	common "github.com/Yulian302/lfusys-services-commons"
	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/Yulian302/lfusys-services-commons/logger"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth"
	_ "github.com/Yulian302/lfusys-services-gateway/docs"
	"github.com/Yulian302/lfusys-services-gateway/files"
	"github.com/Yulian302/lfusys-services-gateway/logging"
	"github.com/Yulian302/lfusys-services-gateway/routers"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/Yulian302/lfusys-services-gateway/uploads"
	_ "github.com/joho/godotenv/autoload"
)

// @title LFU Sys API
// @version 1.0
// @description LFU Sys API gateway
// @swagger 2.0

// @license.name Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
func main() {
	cfg := common.LoadConfig()

	if err := cfg.AWSConfig.ValidateSecrets(); err != nil {
		log.Fatal("aws security credentials were not found")
	}

	// create db client
	awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(cfg.AWSConfig.Region))
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}
	client := dynamodb.NewFromConfig(awsCfg)
	userStore := store.NewUserStore(client, cfg.DynamoDBConfig.UsersTableName)
	uploadsStore := store.NewUploadsStore(client, cfg.DynamoDBConfig.UploadsTableName)

	r := gin.New()

	r.Use(cors.New(
		cors.Config{
			AllowOrigins:     []string{"http://localhost:3000", "http://frontend:3000"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
			AllowCredentials: true,
		},
	))

	// logging reqs/resp
	baseLogger := logger.CreateLogger(cfg.Env)
	r.Use(logging.LoggerMiddleware(baseLogger))

	// tracing
	if cfg.Tracing {
		tp, err := common.StartTracing()
		if err != nil {
			log.Fatalf("failed to start tracing: %v", err)
		}
		r.Use(otelgin.Middleware("gateway"))
		defer func() { _ = tp.Shutdown(context.Background()) }()
	}

	r.GET("/test", func(ctx *gin.Context) {
		responses.JSONSuccess(ctx, "ok")
	})

	// tel
	clientHandler := otelgrpc.NewClientHandler(
		otelgrpc.WithMessageEvents(otelgrpc.ReceivedEvents), // Record message events
	)

	conn, err := grpc.NewClient(cfg.ServiceConfig.SessionGRPCUrl, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(clientHandler))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	clientStub := pb.NewUploaderClient(conn)
	uploadsService := services.NewUploadsService(uploadsStore, clientStub)
	uploadsHandler := uploads.NewUploadsHandler(uploadsService)
	routers.RegisterUploadsRoutes(uploadsHandler, cfg.JWTConfig.SecretKey, r)

	authService := services.NewAuthServiceImpl(userStore, cfg.JWTConfig.SecretKey, cfg.JWTConfig.RefreshSecretKey)
	authHandler := auth.NewAuthHandler(authService)
	routers.RegisterAuthRoutes(authHandler, cfg.JWTConfig.SecretKey, r)

	fileService := services.NewFileServiceImpl(clientStub)
	fileHandler := files.NewFileHandler(fileService)
	routers.RegisterFileRoutes(fileHandler, cfg.JWTConfig.SecretKey, r)

	if cfg.Env != "PROD" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	r.Run(cfg.ServiceConfig.GatewayAddr)
}
