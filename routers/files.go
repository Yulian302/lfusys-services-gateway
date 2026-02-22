package routers

import (
	"github.com/Yulian302/lfusys-services-gateway/auth"
	"github.com/Yulian302/lfusys-services-gateway/files"

	"github.com/gin-gonic/gin"
)

func RegisterFileRoutes(h *files.FileHandler, jwtSecret string, route *gin.RouterGroup) {
	files := route.Group("/files")
	files.Use(auth.JWTMiddleware(jwtSecret, h.Logger))

	files.GET("/", h.GetFiles)
	files.DELETE("/:fileId", h.DeleteFile)
	files.GET("/:fileId/download", h.GetDownloadURL)
}
