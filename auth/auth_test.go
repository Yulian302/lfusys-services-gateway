package auth_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	common "github.com/Yulian302/lfusys-services-commons"
	"github.com/Yulian302/lfusys-services-commons/crypt"
	"github.com/Yulian302/lfusys-services-commons/test"
	"github.com/Yulian302/lfusys-services-commons/test/mocks"
	"github.com/Yulian302/lfusys-services-gateway/auth"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	authtypes "github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/Yulian302/lfusys-services-gateway/routers"
	"github.com/Yulian302/lfusys-services-gateway/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	cfg       common.Config
	mockStore *mocks.MockDynamoDbStore
	r         *gin.Engine
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	cfg = common.LoadConfig()

	mockStore = &mocks.MockDynamoDbStore{}

	r = gin.Default()

	authService := services.NewAuthServiceImpl(mockStore, cfg.JWTConfig.SecretKey, cfg.JWTConfig.RefreshSecretKey)
	authHandler := auth.NewAuthHandler(authService)
	routers.RegisterAuthRoutes(authHandler, cfg.JWTConfig.SecretKey, r)

	os.Exit(m.Run())
}

func TestLogin_Success(t *testing.T) {
	mockStore.ResetMock()

	hashed, salt := crypt.HashPasswordWithSalt("password123")

	mockStore.On(
		"GetByEmail",
		mock.Anything,
		"test@gmail.com",
	).Return(
		&types.User{
			Salt: salt,
			RegisterUser: types.RegisterUser{
				Email:    "test@gmail.com",
				Password: hashed,
			},
		},
		nil,
	)

	reqBody := authtypes.LoginUser{
		Email:    "test@gmail.com",
		Password: "password123",
	}
	body, _ := json.Marshal(reqBody)

	w := test.PerformRequest(
		r,
		t,
		"POST",
		"/auth/login",
		bytes.NewReader(body),
		[]string{"Content-Type: application/json"},
		false,
		"",
		"",
	)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "login successful")

	mockStore.AssertExpectations(t)
}

func TestRegister_Success(t *testing.T) {
	mockStore.ResetMock()

	mockStore.On(
		"Create",
		mock.Anything,
		mock.AnythingOfType("types.User"),
	).Return(
		nil,
	)

	reqBody := types.RegisterUser{
		Name:     "Test",
		Email:    "test@gmail.com",
		Password: "password123",
	}
	body, _ := json.Marshal(reqBody)

	w := test.PerformRequest(
		r,
		t,
		"POST",
		"/auth/register",
		bytes.NewReader(body),
		[]string{"Content-Type: application/json"},
		false,
		"",
		"",
	)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockStore.AssertExpectations(t)
}
