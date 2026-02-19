package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/Yulian302/lfusys-services-gateway/docs"
	_ "github.com/joho/godotenv/autoload"
)

// @title LFU Sys API
// @version 1.0
// @description LFU Sys API gateway
// @swagger 2.0

// @license.name Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer stop()

	app, err := SetupApp()
	if err != nil {
		app.Logger.Error("app initialization failed", "reason", "wrong_setup")
		os.Exit(1)
	}

	router := BuildRouter(app)

	go func() {
		if err := app.Run(router); err != nil {
			app.Logger.Error("server stopped", "err", err.Error())
		}
	}()

	<-ctx.Done()

	app.Logger.Info("shutdown signal received")
	shutDownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.Shutdown(shutDownContext); err != nil {
		app.Logger.Error("graceful shutdown failed", "err", err.Error())
	}

	app.Logger.Info("server exited properly")
}
