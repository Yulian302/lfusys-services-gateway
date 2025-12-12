package routers

import (
	"github.com/Yulian302/lfusys-services-gateway/uploads"
	"github.com/gin-gonic/gin"
)

func RegisterUploadsRoutes(h *uploads.UploadsHandler, route *gin.Engine) {
	uploads := route.Group("/uploads")

	uploads.POST("/start", h.StartUpload)
}
