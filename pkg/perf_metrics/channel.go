package perfmetrics

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

type channelBucketKey struct {
	channelID int
	model     string
	bucketTs  int64
}

type channelCounters struct {
	attemptCount       int64
	successCount       int64
	totalLatencyMs     int64
	ttftSumMs          int64
	ttftCount          int64
	outputTokens       int64
	generationMs       int64
	cacheReportCount   int64
	cacheHitCount      int64
	cachedInputTokens  int64
	logicalInputTokens int64
}

type atomicChannelBucket struct {
	attemptCount       atomic.Int64
	successCount       atomic.Int64
	totalLatencyMs     atomic.Int64
	ttftSumMs          atomic.Int64
	ttftCount          atomic.Int64
	outputTokens       atomic.Int64
	generationMs       atomic.Int64
	cacheReportCount   atomic.Int64
	cacheHitCount      atomic.Int64
	cachedInputTokens  atomic.Int64
	logicalInputTokens atomic.Int64
}

var channelHotBuckets sync.Map

func RecordRelaySampleWithUsage(info *relaycommon.RelayInfo, success bool, usage *dto.Usage, billed bool) {
	if !perf_metrics_setting.GetSetting().Enabled || info == nil || info.ChannelId == 0 || info.OriginModelName == "" {
		return
	}
	latencyMs := time.Since(info.UpstreamRequestStartTime).Milliseconds()
	if info.UpstreamRequestStartTime.IsZero() {
		latencyMs = time.Since(info.StartTime).Milliseconds()
	}
	if latencyMs < 0 {
		latencyMs = 0
	}
	cachedInput, logicalInput := int64(0), int64(0)
	if success && billed {
		cachedInput = usageCachedTokens(usage)
		logicalInput = usageLogicalInputTokens(usage, info.GetFinalRequestRelayFormat() == relaytypes.RelayFormatClaude)
	}
	sample := channelSample{
		channelID:     info.ChannelId,
		model:         info.OriginModelName,
		latencyMs:     latencyMs,
		success:       success,
		outputTokens:  usageOutputTokens(usage),
		generationMs:  generationDuration(info, latencyMs),
		ttftMs:        attemptTtft(info),
		cacheReported: success && billed && usage != nil && cacheUsageSupported(info.GetFinalRequestRelayFormat()),
		cacheHit:      success && billed && usageHasCache(usage),
		cachedInput:   cachedInput,
		logicalInput:  logicalInput,
	}
	recordChannelSample(sample)
}

type channelSample struct {
	channelID                        int
	model                            string
	latencyMs, ttftMs, generationMs  int64
	outputTokens                     int64
	success, cacheReported, cacheHit bool
	cachedInput, logicalInput        int64
}

func RecordChannelFailure(info *relaycommon.RelayInfo) {
	if info == nil || info.UpstreamRequestStartTime.IsZero() {
		return
	}
	RecordRelaySampleWithUsage(info, false, nil, false)
}

func RecordRealtimeRelaySampleWithUsage(info *relaycommon.RelayInfo, usage *dto.RealtimeUsage) {
	if usage == nil {
		RecordRelaySampleWithUsage(info, true, nil, false)
		return
	}
	inputDetails := usage.InputTokenDetails
	RecordRelaySampleWithUsage(info, true, &dto.Usage{
		PromptTokens:           usage.InputTokens,
		CompletionTokens:       usage.OutputTokens,
		TotalTokens:            usage.TotalTokens,
		PromptTokensDetails:    inputDetails,
		InputTokens:            usage.InputTokens,
		OutputTokens:           usage.OutputTokens,
		InputTokensDetails:     &inputDetails,
		CompletionTokenDetails: usage.OutputTokenDetails,
	}, true)
}

func recordChannelSample(sample channelSample) {
	recordChannelSampleAt(sample, time.Now().Unix())
}

func recordChannelSampleAt(sample channelSample, now int64) {
	key := channelBucketKey{channelID: sample.channelID, model: sample.model, bucketTs: channelBucketStart(now)}
	actual, _ := channelHotBuckets.LoadOrStore(key, &atomicChannelBucket{})
	bucket := actual.(*atomicChannelBucket)
	bucket.attemptCount.Add(1)
	if sample.success {
		bucket.successCount.Add(1)
	}
	if sample.latencyMs > 0 {
		bucket.totalLatencyMs.Add(sample.latencyMs)
	}
	if sample.ttftMs >= 0 {
		bucket.ttftSumMs.Add(sample.ttftMs)
		bucket.ttftCount.Add(1)
	}
	if sample.outputTokens > 0 && sample.generationMs > 0 {
		bucket.outputTokens.Add(sample.outputTokens)
		bucket.generationMs.Add(sample.generationMs)
	}
	if sample.cacheReported {
		bucket.cacheReportCount.Add(1)
	}
	if sample.cacheHit {
		bucket.cacheHitCount.Add(1)
	}
	if sample.cachedInput > 0 {
		bucket.cachedInputTokens.Add(sample.cachedInput)
	}
	if sample.logicalInput > 0 {
		bucket.logicalInputTokens.Add(sample.logicalInput)
	}
	recordChannelRedis(key, sample)
}

func channelBucketStart(ts int64) int64 { return ts - ts%300 }

func snapshotChannelBucket(bucket *atomicChannelBucket) channelCounters {
	return channelCounters{
		attemptCount: bucket.attemptCount.Load(), successCount: bucket.successCount.Load(),
		totalLatencyMs: bucket.totalLatencyMs.Load(), ttftSumMs: bucket.ttftSumMs.Load(),
		ttftCount: bucket.ttftCount.Load(), outputTokens: bucket.outputTokens.Load(),
		generationMs: bucket.generationMs.Load(), cacheReportCount: bucket.cacheReportCount.Load(),
		cacheHitCount: bucket.cacheHitCount.Load(), cachedInputTokens: bucket.cachedInputTokens.Load(),
		logicalInputTokens: bucket.logicalInputTokens.Load(),
	}
}

func drainChannelBucket(bucket *atomicChannelBucket) channelCounters {
	return channelCounters{
		attemptCount: bucket.attemptCount.Swap(0), successCount: bucket.successCount.Swap(0),
		totalLatencyMs: bucket.totalLatencyMs.Swap(0), ttftSumMs: bucket.ttftSumMs.Swap(0),
		ttftCount: bucket.ttftCount.Swap(0), outputTokens: bucket.outputTokens.Swap(0),
		generationMs: bucket.generationMs.Swap(0), cacheReportCount: bucket.cacheReportCount.Swap(0),
		cacheHitCount: bucket.cacheHitCount.Swap(0), cachedInputTokens: bucket.cachedInputTokens.Swap(0),
		logicalInputTokens: bucket.logicalInputTokens.Swap(0),
	}
}

func flushChannelCompletedBuckets() {
	current := channelBucketStart(time.Now().Unix())
	channelHotBuckets.Range(func(key, value any) bool {
		k := key.(channelBucketKey)
		if k.bucketTs >= current {
			return true
		}
		counters := drainChannelBucket(value.(*atomicChannelBucket))
		if counters.attemptCount == 0 {
			channelHotBuckets.Delete(key)
			return true
		}
		err := model.UpsertChannelPerfMetric(&model.ChannelPerfMetric{
			ChannelId: k.channelID, ModelName: k.model, BucketTs: k.bucketTs,
			AttemptCount: counters.attemptCount, SuccessCount: counters.successCount,
			TotalLatencyMs: counters.totalLatencyMs, TtftSumMs: counters.ttftSumMs,
			TtftCount: counters.ttftCount, OutputTokens: counters.outputTokens,
			GenerationMs: counters.generationMs, CacheReportCount: counters.cacheReportCount,
			CacheHitCount: counters.cacheHitCount, CachedInputTokens: counters.cachedInputTokens,
			LogicalInputTokens: counters.logicalInputTokens,
		})
		if err != nil {
			bucket := value.(*atomicChannelBucket)
			bucket.attemptCount.Add(counters.attemptCount)
			bucket.successCount.Add(counters.successCount)
			bucket.totalLatencyMs.Add(counters.totalLatencyMs)
			bucket.ttftSumMs.Add(counters.ttftSumMs)
			bucket.ttftCount.Add(counters.ttftCount)
			bucket.outputTokens.Add(counters.outputTokens)
			bucket.generationMs.Add(counters.generationMs)
			bucket.cacheReportCount.Add(counters.cacheReportCount)
			bucket.cacheHitCount.Add(counters.cacheHitCount)
			bucket.cachedInputTokens.Add(counters.cachedInputTokens)
			bucket.logicalInputTokens.Add(counters.logicalInputTokens)
			return true
		}
		channelHotBuckets.Delete(key)
		return true
	})
}

type ChannelPerformance struct {
	ChannelID        int                        `json:"channel_id"`
	ChannelName      string                     `json:"channel_name"`
	ChannelType      int                        `json:"channel_type"`
	AttemptCount     int64                      `json:"attempt_count"`
	SuccessCount     int64                      `json:"success_count"`
	SuccessRate      float64                    `json:"success_rate"`
	AvgLatencyMs     int64                      `json:"avg_latency_ms"`
	AvgTtftMs        int64                      `json:"avg_ttft_ms"`
	AvgTps           float64                    `json:"avg_tps"`
	CacheHitRate     *float64                   `json:"cache_hit_rate"`
	CacheRate        *float64                   `json:"cache_rate"`
	CacheReportCount int64                      `json:"cache_report_count"`
	CacheHitCount    int64                      `json:"cache_hit_count"`
	CachedInput      int64                      `json:"cached_input_tokens"`
	LogicalInput     int64                      `json:"logical_input_tokens"`
	ActiveModelCount int                        `json:"active_model_count"`
	Series           []ChannelPerformanceBucket `json:"series"`
}

type ChannelPerformanceBucket struct {
	StartTs        int64    `json:"start_ts"`
	EndTs          int64    `json:"end_ts"`
	AttemptCount   int64    `json:"attempt_count"`
	SuccessCount   int64    `json:"success_count"`
	TotalLatencyMs int64    `json:"total_latency_ms"`
	SuccessRate    float64  `json:"success_rate"`
	CacheHitRate   *float64 `json:"cache_hit_rate"`
	CacheRate      *float64 `json:"cache_rate"`
}

type ChannelPerformanceResult struct {
	UpdatedAt int64                `json:"updated_at"`
	Channels  []ChannelPerformance `json:"channels"`
}

func QueryChannelPerformance(hours int) (ChannelPerformanceResult, error) {
	if hours != 1 && hours != 24 && hours != 168 {
		hours = 24
	}
	end := time.Now().Unix()
	interval, bucketCount, start := channelRangeConfig(hours, end)
	rows, err := model.GetChannelPerfMetrics(start, end)
	if err != nil {
		return ChannelPerformanceResult{}, err
	}
	merged := make(map[channelBucketKey]channelCounters)
	for _, row := range rows {
		key := channelBucketKey{channelID: row.ChannelId, model: row.ModelName, bucketTs: row.BucketTs}
		mergeChannelCounters(merged, key, channelCounters{attemptCount: row.AttemptCount, successCount: row.SuccessCount, totalLatencyMs: row.TotalLatencyMs, ttftSumMs: row.TtftSumMs, ttftCount: row.TtftCount, outputTokens: row.OutputTokens, generationMs: row.GenerationMs, cacheReportCount: row.CacheReportCount, cacheHitCount: row.CacheHitCount, cachedInputTokens: row.CachedInputTokens, logicalInputTokens: row.LogicalInputTokens})
	}
	channels, err := model.GetEnabledChannelSummaries()
	if err != nil {
		return ChannelPerformanceResult{}, err
	}
	redisMerged := mergeRedisActiveChannelBuckets(merged, channels, start, end)
	activeBucket := channelBucketStart(time.Now().Unix())
	channelHotBuckets.Range(func(key, value any) bool {
		k := key.(channelBucketKey)
		if k.bucketTs < start || k.bucketTs > end || (redisMerged && k.bucketTs == activeBucket) {
			return true
		}
		mergeChannelCounters(merged, k, snapshotChannelBucket(value.(*atomicChannelBucket)))
		return true
	})
	byChannel := make(map[int][]channelBucketKey)
	for key := range merged {
		byChannel[key.channelID] = append(byChannel[key.channelID], key)
	}
	result := ChannelPerformanceResult{UpdatedAt: time.Now().Unix(), Channels: make([]ChannelPerformance, 0, len(channels))}
	for _, channel := range channels {
		series := make([]ChannelPerformanceBucket, 0, bucketCount)
		var total channelCounters
		models := map[string]struct{}{}
		for i := 0; i < bucketCount; i++ {
			bucketStart := start + int64(i)*interval
			var bucket channelCounters
			for _, key := range byChannel[channel.Id] {
				if key.bucketTs < bucketStart || key.bucketTs >= bucketStart+interval {
					continue
				}
				value := merged[key]
				bucket = addChannelCounters(bucket, value)
				if value.attemptCount > 0 {
					models[key.model] = struct{}{}
				}
			}
			total = addChannelCounters(total, bucket)
			series = append(series, buildChannelBucket(bucketStart, bucketStart+interval, bucket))
		}
		result.Channels = append(result.Channels, buildChannelPerformance(channel, total, len(models), series))
	}
	return result, nil
}

func recordChannelRedis(key channelBucketKey, sample channelSample) {
	if !common.RedisEnabled || common.RDB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	metricKey := channelRedisBucketKey(key)
	modelsKey := channelRedisModelsKey(key.channelID, key.bucketTs)
	pipe := common.RDB.TxPipeline()
	pipe.SAdd(ctx, modelsKey, key.model)
	pipe.HIncrBy(ctx, metricKey, "attempt", 1)
	if sample.success {
		pipe.HIncrBy(ctx, metricKey, "ok", 1)
	}
	if sample.latencyMs > 0 {
		pipe.HIncrBy(ctx, metricKey, "latency", sample.latencyMs)
	}
	if sample.ttftMs >= 0 {
		pipe.HIncrBy(ctx, metricKey, "ttft", sample.ttftMs)
		pipe.HIncrBy(ctx, metricKey, "ttft_n", 1)
	}
	if sample.outputTokens > 0 && sample.generationMs > 0 {
		pipe.HIncrBy(ctx, metricKey, "output", sample.outputTokens)
		pipe.HIncrBy(ctx, metricKey, "generation", sample.generationMs)
	}
	if sample.cacheReported {
		pipe.HIncrBy(ctx, metricKey, "cache_report", 1)
	}
	if sample.cacheHit {
		pipe.HIncrBy(ctx, metricKey, "cache_hit", 1)
	}
	if sample.cachedInput > 0 {
		pipe.HIncrBy(ctx, metricKey, "cached_input", sample.cachedInput)
	}
	if sample.logicalInput > 0 {
		pipe.HIncrBy(ctx, metricKey, "logical_input", sample.logicalInput)
	}
	pipe.Expire(ctx, metricKey, 30*time.Minute)
	pipe.Expire(ctx, modelsKey, 30*time.Minute)
	_, _ = pipe.Exec(ctx)
}

func mergeRedisActiveChannelBuckets(merged map[channelBucketKey]channelCounters, channels []model.EnabledChannel, start, end int64) bool {
	if !common.RedisEnabled || common.RDB == nil {
		return false
	}
	active := channelBucketStart(time.Now().Unix())
	if active < start || active > end {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	modelNames := make(map[int][]string, len(channels))
	pipe := common.RDB.Pipeline()
	commands := make(map[int]interface{ Result() ([]string, error) }, len(channels))
	for _, channel := range channels {
		commands[channel.Id] = pipe.SMembers(ctx, channelRedisModelsKey(channel.Id, active))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return false
	}
	for channelID, command := range commands {
		names, err := command.Result()
		if err != nil {
			return false
		}
		modelNames[channelID] = names
	}

	metricPipe := common.RDB.Pipeline()
	type redisMetricCommand struct {
		key     channelBucketKey
		command interface {
			Result() (map[string]string, error)
		}
	}
	metricCommands := make([]redisMetricCommand, 0)
	for channelID, names := range modelNames {
		for _, name := range names {
			key := channelBucketKey{channelID: channelID, model: name, bucketTs: active}
			metricCommands = append(metricCommands, redisMetricCommand{key: key, command: metricPipe.HGetAll(ctx, channelRedisBucketKey(key))})
		}
	}
	if len(metricCommands) == 0 {
		return true
	}
	if _, err := metricPipe.Exec(ctx); err != nil {
		return false
	}
	for _, item := range metricCommands {
		values, err := item.command.Result()
		if err != nil {
			return false
		}
		mergeChannelCounters(merged, item.key, channelRedisCounters(values))
	}
	return true
}

func channelRedisBucketKey(key channelBucketKey) string {
	return fmt.Sprintf("channel-perf:%d:%d:%s", key.channelID, key.bucketTs, key.model)
}

func channelRedisModelsKey(channelID int, bucketTs int64) string {
	return fmt.Sprintf("channel-perf-models:%d:%d", channelID, bucketTs)
}

func channelRedisCounters(values map[string]string) channelCounters {
	return channelCounters{
		attemptCount: parseRedisInt(values["attempt"]), successCount: parseRedisInt(values["ok"]),
		totalLatencyMs: parseRedisInt(values["latency"]), ttftSumMs: parseRedisInt(values["ttft"]),
		ttftCount: parseRedisInt(values["ttft_n"]), outputTokens: parseRedisInt(values["output"]),
		generationMs: parseRedisInt(values["generation"]), cacheReportCount: parseRedisInt(values["cache_report"]),
		cacheHitCount: parseRedisInt(values["cache_hit"]), cachedInputTokens: parseRedisInt(values["cached_input"]),
		logicalInputTokens: parseRedisInt(values["logical_input"]),
	}
}

func channelRangeConfig(hours int, now int64) (interval int64, bucketCount int, start int64) {
	interval = 300
	bucketCount = 12
	if hours == 24 {
		interval = 1800
		bucketCount = 48
	}
	if hours == 168 {
		interval = 21600
		bucketCount = 28
	}
	lastBucket := now - now%interval
	return interval, bucketCount, lastBucket - int64(bucketCount-1)*interval
}

func mergeChannelCounters(target map[channelBucketKey]channelCounters, key channelBucketKey, value channelCounters) {
	target[key] = addChannelCounters(target[key], value)
}

func addChannelCounters(current, value channelCounters) channelCounters {
	current.attemptCount += value.attemptCount
	current.successCount += value.successCount
	current.totalLatencyMs += value.totalLatencyMs
	current.ttftSumMs += value.ttftSumMs
	current.ttftCount += value.ttftCount
	current.outputTokens += value.outputTokens
	current.generationMs += value.generationMs
	current.cacheReportCount += value.cacheReportCount
	current.cacheHitCount += value.cacheHitCount
	current.cachedInputTokens += value.cachedInputTokens
	current.logicalInputTokens += value.logicalInputTokens
	return current
}

func buildChannelBucket(start, end int64, value channelCounters) ChannelPerformanceBucket {
	return ChannelPerformanceBucket{StartTs: start, EndTs: end, AttemptCount: value.attemptCount, SuccessCount: value.successCount, TotalLatencyMs: value.totalLatencyMs, SuccessRate: rate(value.successCount, value.attemptCount), CacheHitRate: cacheRate(value.cacheHitCount, value.cacheReportCount), CacheRate: cacheRate(value.cachedInputTokens, value.logicalInputTokens)}
}

func buildChannelPerformance(channel model.EnabledChannel, value channelCounters, modelCount int, series []ChannelPerformanceBucket) ChannelPerformance {
	return ChannelPerformance{ChannelID: channel.Id, ChannelName: channel.Name, ChannelType: channel.Type, AttemptCount: value.attemptCount, SuccessCount: value.successCount, SuccessRate: rate(value.successCount, value.attemptCount), AvgLatencyMs: avgInt(value.totalLatencyMs, value.attemptCount), AvgTtftMs: avgInt(value.ttftSumMs, value.ttftCount), AvgTps: channelAvgTps(value.outputTokens, value.generationMs), CacheHitRate: cacheRate(value.cacheHitCount, value.cacheReportCount), CacheRate: cacheRate(value.cachedInputTokens, value.logicalInputTokens), CacheReportCount: value.cacheReportCount, CacheHitCount: value.cacheHitCount, CachedInput: value.cachedInputTokens, LogicalInput: value.logicalInputTokens, ActiveModelCount: modelCount, Series: series}
}

func rate(n, d int64) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d) * 100
}
func cacheRate(n, d int64) *float64 {
	if d == 0 {
		return nil
	}
	v := rate(n, d)
	return &v
}
func avgInt(sum, count int64) int64 {
	if count == 0 {
		return 0
	}
	return sum / count
}
func channelAvgTps(tokens, ms int64) float64 {
	if tokens == 0 || ms == 0 {
		return 0
	}
	return float64(tokens) / (float64(ms) / 1000)
}
func attemptTtft(info *relaycommon.RelayInfo) int64 {
	if info.IsStream && info.HasSendResponse() && !info.UpstreamRequestStartTime.IsZero() {
		return info.FirstResponseTime.Sub(info.UpstreamRequestStartTime).Milliseconds()
	}
	return -1
}
func generationDuration(info *relaycommon.RelayInfo, latency int64) int64 {
	if info.IsStream && info.HasSendResponse() && !info.UpstreamRequestStartTime.IsZero() {
		value := time.Since(info.FirstResponseTime).Milliseconds()
		if value > 0 {
			return value
		}
	}
	return latency
}
func usageOutputTokens(usage *dto.Usage) int64 {
	if usage == nil {
		return 0
	}
	if usage.CompletionTokens > 0 {
		return int64(usage.CompletionTokens)
	}
	return int64(usage.OutputTokens)
}
func usageCachedTokens(usage *dto.Usage) int64 {
	if usage == nil {
		return 0
	}
	value := usage.PromptTokensDetails.CachedTokens
	if value == 0 && usage.InputTokensDetails != nil {
		value = usage.InputTokensDetails.CachedTokens
	}
	return int64(value)
}
func usageHasCache(usage *dto.Usage) bool {
	return usageCachedTokens(usage) > 0 || (usage != nil && usage.PromptCacheHitTokens > 0)
}
func usageLogicalInputTokens(usage *dto.Usage, isAnthropic bool) int64 {
	if usage == nil {
		return 0
	}
	value := int64(usage.PromptTokens)
	if isAnthropic || usage.UsageSemantic == dto.BillingUsageSemanticAnthropic {
		value += usageCachedTokens(usage) + int64(usage.PromptTokensDetails.CachedCreationTokens)
	}
	return value
}
func cacheUsageSupported(format relaytypes.RelayFormat) bool {
	switch format {
	case relaytypes.RelayFormatOpenAI, relaytypes.RelayFormatOpenAIResponses, relaytypes.RelayFormatOpenAIResponsesCompaction, relaytypes.RelayFormatOpenAIRealtime, relaytypes.RelayFormatClaude, relaytypes.RelayFormatGemini:
		return true
	default:
		return false
	}
}
