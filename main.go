package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	common "github.com/Yulian302/lfusys-services-commons"
	_ "github.com/joho/godotenv/autoload"
)

var (
	addr string = common.EnvVar("HTTP_ADDR", ":8000")
)

func main() {
	cfg := common.LoadConfig()
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

	routers.Routes(r, clientStub)
}
