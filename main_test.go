package main

import (
	"testing"

	_ "github.com/Yulian302/lfusys-services-gateway/docs"
	"github.com/Yulian302/lfusys-services-gateway/test"
	"github.com/stretchr/testify/assert"
)

func TestPingRoute(t *testing.T) {
	w := test.PerformRequest(t, "GET", "/test", nil)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}
