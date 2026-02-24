package uploads_test

import (
	"os"
	"testing"

	"github.com/Yulian302/lfusys-services-commons/config"
	logger "github.com/Yulian302/lfusys-services-commons/logging"
	"github.com/Yulian302/lfusys-services-commons/test/mocks"
	"github.com/Yulian302/lfusys-services-gateway/routers"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/Yulian302/lfusys-services-gateway/uploads"
	"github.com/gin-gonic/gin"
)

var (
	cfg       config.Config
	mockStore *mocks.MockDynamoDbStore
	r         *gin.Engine
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET_KEY", "test-secret")
	defer os.Unsetenv("JWT_SECRET_KEY")

	gin.SetMode(gin.TestMode)

	r = gin.Default()

	cfg = config.LoadConfig()
	mockStore = &mocks.MockDynamoDbStore{}
	uploadsService := services.NewUploadsService(
		services.UploadsServiceDeps{
			Client:    nil,
			Breaker:   nil,
			ChunkSize: 128 * 1024,
			Logger:    logger.NullLogger{},
		},
	)
	uploadsHandler := uploads.NewUploadsHandler(uploadsService, nil)

	routers.RegisterUploadsRoutes(uploadsHandler, cfg.JWTConfig.SecretKey, &r.RouterGroup)

	os.Exit(m.Run())
}
