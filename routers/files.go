package routers

import (
	"github.com/Yulian302/lfusys-services-gateway/auth"
	"github.com/Yulian302/lfusys-services-gateway/files"

	"github.com/gin-gonic/gin"
)

func RegisterFileRoutes(h *files.FileHandler, jwtSecret string, route *gin.Engine) {
	files := route.Group("/files")

	files.GET("/", auth.JWTMiddleware(jwtSecret), h.GetFiles)
}
