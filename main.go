package main

import (
	"context"
	"log"

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
	app, err := SetupApp()
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}

	router := BuildRouter(app)

	defer app.Shutdown(context.Background())

	if err := app.Run(router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
