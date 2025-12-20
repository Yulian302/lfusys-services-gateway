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
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth"
	_ "github.com/Yulian302/lfusys-services-gateway/docs"
	"github.com/Yulian302/lfusys-services-gateway/routers"
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

	// verify aws credentials
	if cfg.AWS_ACCESS_KEY_ID == "" || cfg.AWS_SECRET_ACCESS_KEY == "" {
		log.Fatal("aws security credentials were not found")
	}

	// create db client
	awsCfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(cfg.AWS_REGION))
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}
	client := dynamodb.NewFromConfig(awsCfg)
	store := store.NewStore(client, cfg.DynamoDBConfig.DynamoDbUsersTableName, cfg.DynamoDBConfig.DynamoDbUploadsTableName)

	r := gin.Default()

	r.Use(cors.New(
		cors.Config{
			AllowOrigins:     []string{"http://localhost:3000", "http://frontend:3000"},
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
			AllowCredentials: true,
		},
	))

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

	conn, err := grpc.NewClient(cfg.SessionsGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(clientHandler))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	clientStub := pb.NewUploaderClient(conn)
	uploadsHandler := uploads.NewUploadsHandler(clientStub, store)
	routers.RegisterUploadsRoutes(uploadsHandler, cfg.JWTConfig.SECRET_KEY, r)

	authHandler := auth.NewAuthHandler(store, &cfg)
	routers.RegisterAuthRoutes(authHandler, cfg.JWTConfig.SECRET_KEY, r)

	if cfg.Env != "PROD" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	r.Run(cfg.HTTPAddr)
}
