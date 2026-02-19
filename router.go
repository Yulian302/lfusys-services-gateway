package main

import (
	"log/slog"
	"strings"
	"time"

	"github.com/Yulian302/lfusys-services-commons/health"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-commons/ratelimit"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth/handlers"
	"github.com/Yulian302/lfusys-services-gateway/files"
	"github.com/Yulian302/lfusys-services-gateway/middleware"
	"github.com/Yulian302/lfusys-services-gateway/routers"
	"github.com/Yulian302/lfusys-services-gateway/uploads"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func BuildRouter(app *App) *gin.Engine {
	r := gin.New()

	httpLogger := logger.CreateHttpLogger(app.Config.Env)

	applyCors(r, app)
	applyLogging(r, httpLogger)
	applyRateLimiting(r, app)
	applyTracing(r, app)
	applySwagger(r, app)

	registerRoutes(r, app)

	return r
}

func applyCors(r *gin.Engine, app *App) {
	origins := strings.Split(app.Config.CorsConfig.Origins, ",")
	r.Use(cors.New(
		cors.Config{
			AllowOrigins:     origins,
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
			AllowCredentials: true,
		},
	))
}

func applyLogging(r *gin.Engine, logger *slog.Logger) {
	r.Use(middleware.Logger(logger))
}

func applyRateLimiting(r *gin.Engine, app *App) {
	rateLimiter := ratelimit.NewRedisRateLimiter(app.Redis)
	r.Use(middleware.RateLimiterMiddleware(rateLimiter, 100, time.Minute))
}

func applyTracing(r *gin.Engine, app *App) {
	if !app.Config.Tracing {
		return
	}

	r.Use(otelgin.Middleware("gateway-service"))
}

func applySwagger(r *gin.Engine, app *App) {
	if app.Config.Env == "PROD" {
		return
	}
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

func registerRoutes(r *gin.Engine, app *App) {
	r.GET("/test", func(ctx *gin.Context) {
		responses.JSONSuccess(ctx, "ok")
	})

	health.RegisterHealthRoutes(
		health.NewHealthHandler(
			app.Services.Stores.uploads,
			app.Services.Stores.users,
		),
		r,
	)

	v1 := routers.ApplyApiVersioning("1", r)

	routers.RegisterAuthRoutes(
		handlers.NewAuthHandler(app.Services.JwtAuth, app.Services.StateManager, app.Logger),
		handlers.NewGithubHandler(app.Config.FrontendURL, app.Config.GithubConfig, app.Services.OAuth,app.Services.StateManager, app.Services.Stores.users, app.Services.Providers.Github, app.Logger),
		handlers.NewGoogleHandler(app.Config.FrontendURL, app.Config.GoogleConfig, app.Services.OAuth,app.Services.StateManager, app.Services.Stores.users, app.Services.Providers.Google, app.Logger),
		app.Config.JWTConfig.SecretKey,
		v1,
	)

	routers.RegisterUploadsRoutes(
		uploads.NewUploadsHandler(app.Services.Uploads, app.Logger),
		app.Config.JWTConfig.SecretKey,
		v1,
	)

	routers.RegisterFileRoutes(
		files.NewFileHandler(app.Services.Files, app.Logger),
		app.Config.JWTConfig.SecretKey,
		v1,
	)
}
