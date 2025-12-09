package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	common "github.com/Yulian302/lfusys-services-commons"
	_ "github.com/joho/godotenv/autoload"
)

var (
	addr string = common.EnvVar("HTTP_ADDR", ":8000")
)

func main() {
	r := gin.Default()

	r.GET("/test", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(clientHandler))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	clientStub := pb.NewGreeterClient(conn)

	routers.Routes(r, clientStub)
}
