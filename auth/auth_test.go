package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	common "github.com/Yulian302/lfusys-services-commons"
	"github.com/Yulian302/lfusys-services-commons/crypt"
	"github.com/Yulian302/lfusys-services-commons/jwt"
	"github.com/Yulian302/lfusys-services-gateway/auth/types"
	authtypes "github.com/Yulian302/lfusys-services-gateway/auth/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	cfg       *common.Config
	r         *gin.Engine
	mockStore *MockDynamoDbStore
)

type MockDynamoDbStore struct {
	mock.Mock
}

func (m *MockDynamoDbStore) GetByEmail(
	ctx context.Context,
	email string,
) (*authtypes.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(*authtypes.User), args.Error(1)
}

func (m *MockDynamoDbStore) Create(
	ctx context.Context,
	user authtypes.User,
) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	cfg = &common.Config{
		JWTConfig: &jwt.JWTConfig{
			SECRET_KEY: "dNlbJEyipTrofVIJOgHWwvVLU4cIAMS6CxkVAHFxHAs=",
		},
		Env: "TEST",
	}

	mockStore = &MockDynamoDbStore{}

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

	mockStore.On(
		"Create",
		mock.Anything,
		mock.AnythingOfType("types.User"),
	).Return(
		nil,
	)

	r = gin.Default()

	handler := NewAuthHandler(mockStore, cfg)

	r.POST("/auth/login", handler.Login)
	r.POST("/auth/register", handler.Register)

	os.Exit(m.Run())
}

func resetMock() {
	mockStore.ExpectedCalls = nil
	mockStore.Calls = nil
}

func TestLogin_Success(t *testing.T) {
	resetMock()

	reqBody := authtypes.LoginUser{
		Email:    "test@gmail.com",
		Password: "password123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "login successful")

	mockStore.AssertExpectations(t)
}

func TestRegister_Success(t *testing.T) {
	resetMock()

	reqBody := types.RegisterUser{
		Name:     "Test",
		Email:    "test@gmail.com",
		Password: "password123",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockStore.AssertExpectations(t)
}
