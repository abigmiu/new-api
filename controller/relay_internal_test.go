package controller

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayAttemptUseTimeSecondsPrefersAttemptStart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyRequestStartTime, time.Now().Add(-10*time.Second))
	common.SetContextKey(ctx, constant.ContextKeyRelayAttemptStartTime, time.Now().Add(-2*time.Second))

	useTime := relayAttemptUseTimeSeconds(ctx)

	require.GreaterOrEqual(t, useTime, 2)
	assert.LessOrEqual(t, useTime, 3)
}

func TestRelayAttemptUseTimeSecondsFallsBackToRequestStart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyRequestStartTime, time.Now().Add(-4*time.Second))

	useTime := relayAttemptUseTimeSeconds(ctx)

	require.GreaterOrEqual(t, useTime, 4)
	assert.LessOrEqual(t, useTime, 5)
}
