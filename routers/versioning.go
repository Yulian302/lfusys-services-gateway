package routers

import (
	"github.com/gin-gonic/gin"
)

func ApplyApiVersioning(version string, route *gin.Engine) *gin.RouterGroup {
	return route.Group("/api/v" + version)
}
