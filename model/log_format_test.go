package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatUserLogsStripsQuotaSaturation verifies the admin-only quota
// saturation marker (nested under other.admin_info) is removed for non-admin
// log views, since formatUserLogs strips the whole admin_info object.
func TestFormatUserLogsStripsQuotaSaturation(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price": 0.004,
		"admin_info": map[string]interface{}{
			"quota_saturation": map[string]interface{}{
				"op":      "QuotaFromDecimal",
				"kind":    "overflow",
				"clamped": common.MaxQuota,
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	_, hasAdminInfo := parsed["admin_info"]
	require.False(t, hasAdminInfo, "admin_info (and nested quota_saturation) must be stripped for non-admin views")
	// Non-admin billing fields remain visible.
	require.Contains(t, parsed, "model_price")
}

// TestFormatUserLogsStripsPrivilegedRoutingMetadata verifies the admin-only
// upstream routing diagnostics (which channel served the request and why the
// upstream rejected it) are removed for non-admin log views, while
// stream_status stays visible because it explains stream interruptions to the
// log owner.
func TestFormatUserLogsStripsPrivilegedRoutingMetadata(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price":   0.004,
		"reject_reason": "gemini_block_reason=SAFETY",
		"channel_id":    42,
		"channel_name":  "upstream-a",
		"channel_type":  8,
		"stream_status": map[string]interface{}{"status": "error"},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	for _, key := range []string{"reject_reason", "channel_id", "channel_name", "channel_type"} {
		assert.NotContains(t, parsed, key, "%s must be stripped for non-admin views", key)
	}
	assert.Equal(t, map[string]interface{}{"status": "error"}, parsed["stream_status"])
	assert.Contains(t, parsed, "model_price")
}
