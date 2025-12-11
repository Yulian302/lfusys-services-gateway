package routers

import (
	"net/http"

	pb "github.com/Yulian302/lfusys-services-commons/api"
	"github.com/gin-gonic/gin"
)

func Routes(route *gin.Engine, c pb.GreeterClient) {
	uploads := route.Group("/uploads")

	uploads.POST("/start", func(ctx *gin.Context) {
		res, err := c.SayHello(ctx, &pb.HelloReq{
			Name: "Yulian",
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": "could not receive response from server",
			})
			return
		}
		ctx.JSON(http.StatusOK, gin.H{
			"message": res.Msg,
		})
	})
}
