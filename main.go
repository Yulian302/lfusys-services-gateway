package main

import (
	"net/http"

	"github.com/gin-gonic/gin"

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

	r.Run(addr)
}
