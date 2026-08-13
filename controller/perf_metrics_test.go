package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareChannelPerformanceResponseUsesRoleSafeChannelIdentity(t *testing.T) {
	_, _ = setupChannelPreferenceTest(t)
	groups := map[string]struct{}{"gpt-0.1倍率": {}, "vip": {}}
	base := []perfmetrics.ChannelPerformance{
		{ChannelID: 17, ChannelName: "secret-upstream", Groups: []string{"gpt-0.1倍率", "vip"}},
		{ChannelID: 18, ChannelName: "other-upstream", Groups: []string{"vip"}},
	}

	admin := perfmetrics.ChannelPerformanceResult{Channels: append([]perfmetrics.ChannelPerformance(nil), base...)}
	require.NoError(t, prepareChannelPerformanceResponse(&admin, "gpt-0.1倍率", groups, true))
	require.Len(t, admin.Channels, 1)
	assert.Equal(t, 17, admin.Channels[0].ChannelID)
	assert.Equal(t, "secret-upstream", admin.Channels[0].ChannelName)
	assert.Equal(t, "secret-upstream", admin.Channels[0].DisplayName)
	assert.NotEmpty(t, admin.Channels[0].Alias)
	assert.True(t, admin.IsAdmin)
	assert.Equal(t, []string{"gpt-0.1倍率", "vip"}, admin.Groups)

	user := perfmetrics.ChannelPerformanceResult{Channels: append([]perfmetrics.ChannelPerformance(nil), base...)}
	require.NoError(t, prepareChannelPerformanceResponse(&user, "gpt-0.1倍率", groups, false))
	require.Len(t, user.Channels, 1)
	assert.Zero(t, user.Channels[0].ChannelID)
	assert.Empty(t, user.Channels[0].ChannelName)
	assert.Equal(t, "gpt-0.1倍率-"+user.Channels[0].Alias, user.Channels[0].DisplayName)
	assert.False(t, user.IsAdmin)

	payload, err := common.Marshal(user)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "secret-upstream")
	assert.NotContains(t, string(payload), `"channel_id"`)

	channelID, err := service.DecryptChannelAlias("gpt-0.1倍率", user.Channels[0].Alias)
	require.NoError(t, err)
	assert.Equal(t, 17, channelID)
}
