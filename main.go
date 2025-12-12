package main

import (
	"context"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	common "github.com/Yulian302/lfusys-services-commons"
	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/Yulian302/lfusys-services-gateway/auth"
	"github.com/Yulian302/lfusys-services-gateway/routers"
	"github.com/Yulian302/lfusys-services-gateway/store"
	"github.com/Yulian302/lfusys-services-gateway/uploads"
	_ "github.com/joho/godotenv/autoload"
)

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
	store := store.NewStore(client, "users")

	r := gin.Default()

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
		ctx.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})

	// tel
	clientHandler := otelgrpc.NewClientHandler(
		otelgrpc.WithMessageEvents(otelgrpc.ReceivedEvents), // Record message events
	)

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(clientHandler))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	clientStub := pb.NewGreeterClient(conn)
	uploadsHandler := uploads.NewUploadsHandler(clientStub)
	routers.RegisterUploadsRoutes(uploadsHandler, r)

	authHandler := auth.NewAuthHandler(store, &cfg)
	routers.RegisterAuthRoutes(authHandler, r)

	r.Run(cfg.HTTPAddr)
}
