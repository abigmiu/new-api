package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareManagedChannelPerformanceResponseGroupsEnabledBindingsBySupplier(t *testing.T) {
	localChannelA := 17
	localChannelB := 18
	bindings := []model.UpstreamGroupBinding{
		{Id: 3, UpstreamChannelId: 2, UpstreamChannelName: "Supplier B", RemoteGroupName: "Standard", RemoteDescription: "Stable", SaleRatio: "0.089", LocalChannelId: &localChannelA},
		{Id: 2, UpstreamChannelId: 1, UpstreamChannelName: "Supplier A", RemoteGroupName: "Pro", RemoteDescription: "Fast", SaleRatio: "0.119", LocalChannelId: &localChannelB},
		{Id: 1, UpstreamChannelId: 1, UpstreamChannelName: "Supplier A", RemoteGroupName: "Basic", SaleRatio: "0.059", LocalChannelId: &localChannelA},
		{Id: 4, UpstreamChannelId: 3, UpstreamChannelName: "Unbound", RemoteGroupName: "Ignored"},
	}
	result := perfmetrics.ChannelPerformanceResult{
		UpdatedAt: 123,
		Channels: []perfmetrics.ChannelPerformance{
			{ChannelID: localChannelA, AttemptCount: 10, SuccessCount: 9, SuccessRate: 90, AvgLatencyMs: 800},
			{ChannelID: localChannelB, AttemptCount: 20, SuccessCount: 20, SuccessRate: 100, AvgLatencyMs: 400},
		},
	}

	response := prepareManagedChannelPerformanceResponse(result, bindings, true)

	assert.Equal(t, int64(123), response.UpdatedAt)
	require.Len(t, response.Suppliers, 2)
	assert.Equal(t, "Supplier A", response.Suppliers[0].UpstreamChannelName)
	require.Len(t, response.Suppliers[0].Groups, 2)
	assert.Equal(t, "Basic", response.Suppliers[0].Groups[0].GroupName)
	assert.Equal(t, int64(10), response.Suppliers[0].Groups[0].AttemptCount)
	assert.Equal(t, "Pro", response.Suppliers[0].Groups[1].GroupName)
	assert.Equal(t, int64(20), response.Suppliers[0].Groups[1].AttemptCount)
	assert.Equal(t, "Supplier B", response.Suppliers[1].UpstreamChannelName)
	assert.Equal(t, "Stable", response.Suppliers[1].Groups[0].Description)
	assert.Equal(t, "0.089", response.Suppliers[1].Groups[0].SaleRatio)
}

func TestPrepareManagedChannelPerformanceResponseKeepsGroupWithoutMetrics(t *testing.T) {
	localChannelID := 99
	bindings := []model.UpstreamGroupBinding{{
		Id: 1, UpstreamChannelId: 1, UpstreamChannelName: "Supplier", RemoteGroupName: "New Group", LocalChannelId: &localChannelID,
	}}

	response := prepareManagedChannelPerformanceResponse(perfmetrics.ChannelPerformanceResult{}, bindings, true)

	require.Len(t, response.Suppliers, 1)
	require.Len(t, response.Suppliers[0].Groups, 1)
	group := response.Suppliers[0].Groups[0]
	assert.Zero(t, group.AttemptCount)
	assert.Zero(t, group.SuccessRate)
	assert.Empty(t, group.Series)
}

func TestPrepareManagedChannelPerformanceResponseHidesSupplierNameFromNonAdmins(t *testing.T) {
	localChannelID := 17
	bindings := []model.UpstreamGroupBinding{{
		Id: 1, UpstreamChannelId: 5, UpstreamChannelName: "Secret Supplier", RemoteGroupName: "Pro", LocalChannelId: &localChannelID,
	}}
	result := perfmetrics.ChannelPerformanceResult{
		Channels: []perfmetrics.ChannelPerformance{{ChannelID: localChannelID, AttemptCount: 4}},
	}

	response := prepareManagedChannelPerformanceResponse(result, bindings, false)

	require.Len(t, response.Suppliers, 1)
	assert.Equal(t, "渠道5", response.Suppliers[0].UpstreamChannelName)
	require.Len(t, response.Suppliers[0].Groups, 1)
	assert.Equal(t, "Pro", response.Suppliers[0].Groups[0].GroupName)
}
