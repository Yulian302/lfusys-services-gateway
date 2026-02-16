package uploads_test

import (
	"os"
	"testing"

	"github.com/Yulian302/lfusys-services-commons/config"
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

	uploadsService := services.NewUploadsService(mockStore, nil, nil)
	uploadsHandler := uploads.NewUploadsHandler(uploadsService)

	routers.RegisterUploadsRoutes(uploadsHandler, cfg.JWTConfig.SecretKey, &r.RouterGroup)

	os.Exit(m.Run())
}
