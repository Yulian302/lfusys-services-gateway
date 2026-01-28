package main

import (
	"log"
	"strings"
	"time"

	common "github.com/Yulian302/lfusys-services-commons"
	"github.com/Yulian302/lfusys-services-commons/health"
	"github.com/Yulian302/lfusys-services-commons/logger"
	"github.com/Yulian302/lfusys-services-commons/ratelimit"
	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-gateway/auth/handlers"
	"github.com/Yulian302/lfusys-services-gateway/files"
	"github.com/Yulian302/lfusys-services-gateway/logging"
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

	applyCors(r, app)
	applyLogging(r, app)
	applyRateLimiting(r, app)
	applyTracing(r, app)
	applySwagger(r, app)

	registerRoutes(r, app, app.Services)

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

func applyLogging(r *gin.Engine, app *App) {
	baseLogger := logger.CreateLogger(app.Config.Env)
	r.Use(logging.LoggerMiddleware(baseLogger))
}

func applyRateLimiting(r *gin.Engine, app *App) {
	rateLimiter := ratelimit.NewRedisRateLimiter(app.Redis)
	r.Use(middleware.RateLimiterMiddleware(rateLimiter, 100, time.Minute))
}

func applyTracing(r *gin.Engine, app *App) {
	if !app.Config.Tracing {
		return
	}

	tp, err := common.StartTracing()
	if err != nil {
		log.Fatalf("failed to start tracing: %v", err)
	}

	app.TracerProvider = tp
	r.Use(otelgin.Middleware("gateway"))
}

func applySwagger(r *gin.Engine, app *App) {
	if app.Config.Env == "PROD" {
		return
	}
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

func registerRoutes(r *gin.Engine, app *App, s *Services) {
	r.GET("/test", func(ctx *gin.Context) {
		responses.JSONSuccess(ctx, "ok")
	})

	health.RegisterHealthRoutes(
		health.NewHealthHandler(
			s.Stores.uploads,
		),
		r,
	)

	routers.RegisterAuthRoutes(
		handlers.NewAuthHandler(s.Auth),
		handlers.NewGithubHandler(app.Config.FrontendURL, app.Config.GithubConfig, s.Auth, s.Stores.users, s.Providers.Github),
		handlers.NewGoogleHandler(app.Config.FrontendURL, app.Config.GoogleConfig, s.Auth, s.Stores.users, s.Providers.Google),
		app.Config.JWTConfig.SecretKey,
		r,
	)

	routers.RegisterUploadsRoutes(
		uploads.NewUploadsHandler(s.Uploads),
		app.Config.JWTConfig.SecretKey,
		r,
	)

	routers.RegisterFileRoutes(
		files.NewFileHandler(s.Files),
		app.Config.JWTConfig.SecretKey,
		r,
	)
}
