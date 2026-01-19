package uploads_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/Yulian302/lfusys-services-commons/config"
	"github.com/Yulian302/lfusys-services-commons/test"
	"github.com/Yulian302/lfusys-services-commons/test/mocks"
	"github.com/Yulian302/lfusys-services-gateway/routers"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/Yulian302/lfusys-services-gateway/uploads"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	cfg       config.Config
	mockStore *mocks.MockDynamoDbStore
	r         *gin.Engine
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	r = gin.Default()

	cfg = config.LoadConfig()
	mockStore = &mocks.MockDynamoDbStore{}

	uploadsService := services.NewUploadsService(mockStore, nil, nil)
	uploadsHandler := uploads.NewUploadsHandler(uploadsService)

	routers.RegisterUploadsRoutes(uploadsHandler, cfg.JWTConfig.SecretKey, r)

	os.Exit(m.Run())
}

func TestCreateUploadSession_AlreadyExists(t *testing.T) {
	mockStore.ResetMock()

	mockStore.On(
		"FindExisting",
		mock.Anything,
		mock.MatchedBy(func(email string) bool {
			return email != ""
		}),
	).Return(
		true,
		nil,
	)

	reqBody := uploads.UploadRequest{
		FileSize: 100,
	}
	body, _ := json.Marshal(reqBody)

	w := test.PerformRequest(
		r,
		t,
		"POST",
		"/uploads/start",
		bytes.NewReader(body),
		[]string{"Content-Type: application/json"},
		true,
		cfg.JWTConfig.SecretKey,
		"test@gmail.com",
	)

	assert.Equal(t, 409, w.Code)
}
