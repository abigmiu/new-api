package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTextOtherInfoIncludesManagedGroupPriceSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyTokenGroupBindingId, int64(12))
	common.SetContextKey(ctx, constant.ContextKeyTokenGroupPriceVersion, int64(4))
	common.SetContextKey(ctx, constant.ContextKeyTokenGroupSourceRatio, "0.075")
	common.SetContextKey(ctx, constant.ContextKeyTokenGroupSaleRatio, "0.089")
	now := time.Now()
	relayInfo := &relaycommon.RelayInfo{
		StartTime:         now,
		FirstResponseTime: now,
		ChannelMeta:       &relaycommon.ChannelMeta{},
	}

	other := GenerateTextOtherInfo(ctx, relayInfo, 1, 0.089, 1, 0, 1, 0, -1)

	assert.Equal(t, 0.089, other["group_ratio"])
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, int64(12), adminInfo["upstream_group_binding_id"])
	assert.Equal(t, int64(4), adminInfo["upstream_group_price_version"])
	assert.Equal(t, "0.075", adminInfo["upstream_group_source_ratio"])
	assert.Equal(t, "0.089", adminInfo["upstream_group_sale_ratio"])
}
