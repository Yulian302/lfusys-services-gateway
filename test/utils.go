package test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/gin-gonic/gin"
)

func CreateTestRouter() *gin.Engine {
	r := gin.Default()
	gin.SetMode(gin.TestMode)

	r.GET("/test", func(ctx *gin.Context) { responses.JSONSuccess(ctx, "ok") })

	r.POST("/login")

	return r
}

func PerformRequest(t *testing.T, method, url string, body io.Reader) *httptest.ResponseRecorder {
	r := CreateTestRouter()
	w := httptest.NewRecorder()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	r.ServeHTTP(w, req)
	return w
}
