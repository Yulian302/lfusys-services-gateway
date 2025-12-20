package routers

import (
	"github.com/Yulian302/lfusys-services-gateway/auth"
	authmid "github.com/Yulian302/lfusys-services-gateway/auth"
	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(h *auth.AuthHandler, jwtSecret string, route *gin.Engine) {
	auth := route.Group("/auth")

	auth.GET("/me", authmid.JWTMiddleware(jwtSecret), h.Me)
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)
	auth.POST("/refresh", h.Refresh)
	auth.POST("/logout", h.Logout)
}
