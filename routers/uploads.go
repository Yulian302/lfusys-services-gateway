package routers

import (
	"github.com/Yulian302/lfusys-services-gateway/auth"
	"github.com/Yulian302/lfusys-services-gateway/uploads"
	"github.com/gin-gonic/gin"
)

func RegisterUploadsRoutes(h *uploads.UploadsHandler, jwtSecret string, route *gin.Engine) {
	uploads := route.Group("/uploads")

	uploads.Use(auth.JWTMiddleware(jwtSecret))
	uploads.POST("/start", h.StartUpload)
	uploads.GET("/:uploadId/status", h.GetUploadStatus)
}
