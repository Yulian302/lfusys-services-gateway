package main

import (
	"os"
	"testing"

	"github.com/Yulian302/lfusys-services-commons/responses"
	"github.com/Yulian302/lfusys-services-commons/test"
	_ "github.com/Yulian302/lfusys-services-gateway/docs"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

var (
	r *gin.Engine
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	r = gin.Default()

	r.GET("/test", func(ctx *gin.Context) { responses.JSONSuccess(ctx, "ok") })

	os.Exit(m.Run())
}

func TestPingRoute(t *testing.T) {
	w := test.PerformRequest(
		r,
		t,
		"GET",
		"/test",
		nil,
		nil,
		false,
		"",
		"",
	)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}
