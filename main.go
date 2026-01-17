package main

import (
	"context"
	"log"
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
		log.Fatalf("failed to initialize app: %v", err)
	}

	router := BuildRouter(app)

	go func() {
		if err := app.Run(router); err != nil {
			log.Printf("server stopped: %v", err)
		}
	}()

	<-ctx.Done()

	log.Println("shutdown signal received")
	shutDownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.Shutdown(shutDownContext); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}

	log.Println("server exited properly")

}
