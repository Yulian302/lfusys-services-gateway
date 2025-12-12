package routers

import (
	"github.com/Yulian302/lfusys-services-gateway/auth"
	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(h *auth.AuthHandler, route *gin.Engine) {
	auth := route.Group("/auth")

	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
}
