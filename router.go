package main

import (
	"github.com/Yulian302/lfusys-services-commons/config"
	"github.com/Yulian302/lfusys-services-commons/health"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-commons/middleware"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth/handlers"
	"github.com/Yulian302/lfusys-services-gateway/files"
	"github.com/Yulian302/lfusys-services-gateway/routers"
	"github.com/Yulian302/lfusys-services-gateway/uploads"
	"github.com/gin-gonic/gin"
)

func BuildRouter(app *App) *gin.Engine {
	r := gin.New()

	httpLogger := logger.CreateHttpLogger(app.Config.Env)
	middleware.ApplyLogging(r, httpLogger)

	middleware.ApplyCors(r, app.Config.Cors)
	middleware.ApplyRateLimiting(r, app.Redis, app.Logger)

	if app.Config.Env != config.EnvProduction {
		middleware.ApplyTracing(r, "gateway-service")
		middleware.ApplySwagger(r)
	}

	registerRoutes(r, app)

	return r
}

func registerRoutes(r *gin.Engine, app *App) {
	r.GET("/test", func(ctx *gin.Context) {
		responses.JSONSuccess(ctx, "ok")
	})

	health.RegisterHealthRoutes(
		health.NewHealthHandler(
			app.Logger,
			app.Services.Stores.users,
		),
		r,
	)

	v1 := routers.ApplyApiVersioning("1", r)

	routers.RegisterAuthRoutes(
		handlers.NewAuthHandler(app.Services.JwtAuth, app.Services.StateManager, app.Logger),
		handlers.NewGithubHandler(app.Config.Service.Gateway.FrontendUrl, app.Config.OAuth.Github, app.Services.OAuth, app.Services.StateManager, app.Services.Stores.users, app.Services.Providers.Github, app.Logger),
		handlers.NewGoogleHandler(app.Config.Service.Gateway.FrontendUrl, app.Config.OAuth.Google, app.Services.OAuth, app.Services.StateManager, app.Services.Stores.users, app.Services.Providers.Google, app.Logger),
		app.Config.JWT.SecretKey,
		v1,
		app.Logger,
	)

	routers.RegisterUploadsRoutes(
		uploads.NewUploadsHandler(app.Services.Uploads, app.Logger),
		app.Config.JWT.SecretKey,
		v1,
	)

	routers.RegisterFileRoutes(
		files.NewFileHandler(app.Services.Files, app.Logger),
		app.Config.JWT.SecretKey,
		v1,
	)
}
