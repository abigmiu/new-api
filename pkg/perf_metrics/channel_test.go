package perfmetrics

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func useChannelMetricsRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, client.Ping(context.Background()).Err())
	previousEnabled, previousClient := common.RedisEnabled, common.RDB
	common.RedisEnabled, common.RDB = true, client
	t.Cleanup(func() {
		_ = client.Close()
		common.RedisEnabled, common.RDB = previousEnabled, previousClient
	})
	return server
}

func TestRecordChannelSampleCountsEachAttemptAndCacheCoverage(t *testing.T) {
	channelID := 900001
	key := channelBucketKey{channelID: channelID, model: "test-model", bucketTs: channelBucketStart(1754890000)}
	actual := &atomicChannelBucket{}
	channelHotBuckets.Store(key, actual)
	t.Cleanup(func() { channelHotBuckets.Delete(key) })

	recordChannelSampleAt(channelSample{channelID: channelID, model: "test-model", latencyMs: 1200}, 1754890000)
	recordChannelSampleAt(channelSample{channelID: channelID, model: "test-model", latencyMs: 800, success: true, cacheReported: true, cacheHit: true, cachedInput: 60, logicalInput: 100}, 1754890000)

	counters := snapshotChannelBucket(actual)
	assert.EqualValues(t, 2, counters.attemptCount)
	assert.EqualValues(t, 1, counters.successCount)
	assert.EqualValues(t, 2000, counters.totalLatencyMs)
	assert.EqualValues(t, 1, counters.cacheReportCount)
	assert.EqualValues(t, 1, counters.cacheHitCount)
	assert.EqualValues(t, 60, counters.cachedInputTokens)
	assert.EqualValues(t, 100, counters.logicalInputTokens)
}

func TestRecordRelaySampleWithUsageExcludesUnbilledAndFailedCacheData(t *testing.T) {
	channelID := 900005
	key := channelBucketKey{channelID: channelID, model: "test-model", bucketTs: channelBucketStart(1754890000)}
	actual := &atomicChannelBucket{}
	channelHotBuckets.Store(key, actual)
	t.Cleanup(func() { channelHotBuckets.Delete(key) })

	info := &relaycommon.RelayInfo{
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: channelID},
		OriginModelName: "test-model",
		RelayFormat:     relaytypes.RelayFormatOpenAI,
	}
	info.UpstreamRequestStartTime = time.Unix(1754890000, 0)
	info.StartTime = info.UpstreamRequestStartTime
	usage := &dto.Usage{PromptTokens: 100, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 40}}

	recordUsageSample := func(success, billed bool) {
		cachedInput, logicalInput := int64(0), int64(0)
		if success && billed {
			cachedInput = usageCachedTokens(usage)
			logicalInput = usageLogicalInputTokens(usage, false)
		}
		recordChannelSampleAt(channelSample{
			channelID:     info.ChannelId,
			model:         info.OriginModelName,
			success:       success,
			cacheReported: success && billed && cacheUsageSupported(info.GetFinalRequestRelayFormat()),
			cacheHit:      success && billed && usageHasCache(usage),
			cachedInput:   cachedInput,
			logicalInput:  logicalInput,
		}, 1754890000)
	}
	recordUsageSample(true, false)
	recordUsageSample(false, true)
	recordUsageSample(true, true)

	counters := snapshotChannelBucket(actual)
	assert.EqualValues(t, 3, counters.attemptCount)
	assert.EqualValues(t, 2, counters.successCount)
	assert.EqualValues(t, 1, counters.cacheReportCount)
	assert.EqualValues(t, 1, counters.cacheHitCount)
	assert.EqualValues(t, 40, counters.cachedInputTokens)
	assert.EqualValues(t, 100, counters.logicalInputTokens)
}

func TestRecordChannelFailureIgnoresErrorsBeforeUpstreamRequest(t *testing.T) {
	channelID := 900004
	key := channelBucketKey{channelID: channelID, model: "test-model", bucketTs: channelBucketStart(time.Now().Unix())}
	actual := &atomicChannelBucket{}
	channelHotBuckets.Store(key, actual)
	t.Cleanup(func() { channelHotBuckets.Delete(key) })

	RecordChannelFailure(&relaycommon.RelayInfo{
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: channelID},
		OriginModelName: "test-model",
	})

	assert.Zero(t, snapshotChannelBucket(actual).attemptCount)
}

func TestUsageLogicalInputTokensNormalizesProviderSemantics(t *testing.T) {
	tests := []struct {
		name     string
		usage    *dto.Usage
		expected int64
	}{
		{
			name:     "OpenAI prompt total already includes cached input",
			usage:    &dto.Usage{PromptTokens: 100, UsageSemantic: dto.BillingUsageSemanticOpenAI, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 30}},
			expected: 100,
		},
		{
			name:     "Anthropic prompt total excludes cache reads and writes",
			usage:    &dto.Usage{PromptTokens: 100, UsageSemantic: dto.BillingUsageSemanticAnthropic, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 30, CachedCreationTokens: 20}},
			expected: 150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, usageLogicalInputTokens(tt.usage, tt.usage.UsageSemantic == dto.BillingUsageSemanticAnthropic))
		})
	}
}

func TestChannelRangeConfigReturnsFixedBucketCounts(t *testing.T) {
	now := int64(1754890123)
	tests := []struct {
		hours, count int
		interval     int64
	}{
		{hours: 1, count: 12, interval: 300},
		{hours: 24, count: 48, interval: 1800},
		{hours: 168, count: 28, interval: 21600},
	}

	for _, tt := range tests {
		interval, count, start := channelRangeConfig(tt.hours, now)
		require.Equal(t, tt.interval, interval)
		assert.Equal(t, tt.count, count)
		assert.EqualValues(t, 0, start%interval)
		assert.EqualValues(t, count-1, (now-now%interval-start)/interval)
	}
}

func TestChannelRedisAggregatesActiveBucketAcrossModels(t *testing.T) {
	server := useChannelMetricsRedis(t)
	now := time.Now().Unix()
	channelID := 900002
	modelAKey := channelBucketKey{channelID: channelID, model: "model-a", bucketTs: channelBucketStart(now)}
	modelBKey := channelBucketKey{channelID: channelID, model: "model-b", bucketTs: channelBucketStart(now)}
	t.Cleanup(func() {
		channelHotBuckets.Delete(modelAKey)
		channelHotBuckets.Delete(modelBKey)
	})
	recordChannelSampleAt(channelSample{channelID: channelID, model: "model-a", latencyMs: 120, success: true, cacheReported: true, cacheHit: true, cachedInput: 40, logicalInput: 100}, now)
	recordChannelSampleAt(channelSample{channelID: channelID, model: "model-b", latencyMs: 80}, now)

	merged := make(map[channelBucketKey]channelCounters)
	ok := mergeRedisActiveChannelBuckets(merged, []model.EnabledChannel{{Id: channelID}}, channelBucketStart(now), now)
	require.True(t, ok)
	assert.EqualValues(t, 1, merged[modelAKey].successCount)
	assert.EqualValues(t, 40, merged[modelAKey].cachedInputTokens)
	assert.EqualValues(t, 1, merged[modelBKey].attemptCount)
	assert.Equal(t, 30*time.Minute, server.TTL(channelRedisModelsKey(channelID, channelBucketStart(now))))
}

func TestChannelRedisReadFailureRequestsLocalFallback(t *testing.T) {
	server := useChannelMetricsRedis(t)
	server.Close()

	merged := make(map[channelBucketKey]channelCounters)
	ok := mergeRedisActiveChannelBuckets(merged, []model.EnabledChannel{{Id: 900003}}, channelBucketStart(time.Now().Unix()), time.Now().Unix())

	assert.False(t, ok)
	assert.Empty(t, merged)
}
