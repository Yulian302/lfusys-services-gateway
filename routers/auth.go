package routers

import (
	authmid "github.com/Yulian302/lfusys-services-gateway/auth"
	"github.com/Yulian302/lfusys-services-gateway/auth/handlers"
	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(jwt *handlers.AuthHandler, gh *handlers.GithubHandler, googleh *handlers.GoogleHandler, jwtSecret string, route *gin.Engine) {
	auth := route.Group("/auth")

	auth.GET("/me", authmid.JWTMiddleware(jwtSecret), jwt.Me)
	auth.POST("/register", jwt.Register)
	auth.POST("/login", jwt.Login)
	auth.POST("/refresh", jwt.Refresh)
	auth.POST("/logout", jwt.Logout)

	// oauth2
	auth.POST("/state", jwt.NewState)
	auth.GET("/github/callback", gh.Callback)
	auth.GET("/google/callback", googleh.Callback)
}
