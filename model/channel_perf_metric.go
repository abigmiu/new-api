package model

import (
	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChannelPerfMetric struct {
	Id                 int    `json:"id" gorm:"primaryKey"`
	ChannelId          int    `json:"channel_id" gorm:"uniqueIndex:idx_channel_perf_bucket,priority:1"`
	ModelName          string `json:"model_name" gorm:"size:128;uniqueIndex:idx_channel_perf_bucket,priority:2"`
	BucketTs           int64  `json:"bucket_ts" gorm:"uniqueIndex:idx_channel_perf_bucket,priority:3;index:idx_channel_perf_ts"`
	AttemptCount       int64  `json:"-" gorm:"default:0"`
	SuccessCount       int64  `json:"-" gorm:"default:0"`
	TotalLatencyMs     int64  `json:"-" gorm:"default:0"`
	TtftSumMs          int64  `json:"-" gorm:"default:0"`
	TtftCount          int64  `json:"-" gorm:"default:0"`
	OutputTokens       int64  `json:"-" gorm:"default:0"`
	GenerationMs       int64  `json:"-" gorm:"default:0"`
	CacheReportCount   int64  `json:"-" gorm:"default:0"`
	CacheHitCount      int64  `json:"-" gorm:"default:0"`
	CachedInputTokens  int64  `json:"-" gorm:"default:0"`
	LogicalInputTokens int64  `json:"-" gorm:"default:0"`
}

func (ChannelPerfMetric) TableName() string { return "channel_perf_metrics" }

func UpsertChannelPerfMetric(metric *ChannelPerfMetric) error {
	if metric == nil || metric.AttemptCount == 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "channel_id"}, {Name: "model_name"}, {Name: "bucket_ts"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"attempt_count":        gorm.Expr("channel_perf_metrics.attempt_count + ?", metric.AttemptCount),
			"success_count":        gorm.Expr("channel_perf_metrics.success_count + ?", metric.SuccessCount),
			"total_latency_ms":     gorm.Expr("channel_perf_metrics.total_latency_ms + ?", metric.TotalLatencyMs),
			"ttft_sum_ms":          gorm.Expr("channel_perf_metrics.ttft_sum_ms + ?", metric.TtftSumMs),
			"ttft_count":           gorm.Expr("channel_perf_metrics.ttft_count + ?", metric.TtftCount),
			"output_tokens":        gorm.Expr("channel_perf_metrics.output_tokens + ?", metric.OutputTokens),
			"generation_ms":        gorm.Expr("channel_perf_metrics.generation_ms + ?", metric.GenerationMs),
			"cache_report_count":   gorm.Expr("channel_perf_metrics.cache_report_count + ?", metric.CacheReportCount),
			"cache_hit_count":      gorm.Expr("channel_perf_metrics.cache_hit_count + ?", metric.CacheHitCount),
			"cached_input_tokens":  gorm.Expr("channel_perf_metrics.cached_input_tokens + ?", metric.CachedInputTokens),
			"logical_input_tokens": gorm.Expr("channel_perf_metrics.logical_input_tokens + ?", metric.LogicalInputTokens),
		}),
	}).Create(metric).Error
}

func GetChannelPerfMetrics(startTs, endTs int64) ([]ChannelPerfMetric, error) {
	var metrics []ChannelPerfMetric
	err := DB.Where("bucket_ts >= ? AND bucket_ts <= ?", startTs, endTs).
		Order("bucket_ts ASC").Find(&metrics).Error
	return metrics, err
}

type EnabledChannel struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	Type  int    `json:"type"`
	Group string `json:"group"`
}

func GetEnabledChannelSummaries() ([]EnabledChannel, error) {
	var channels []EnabledChannel
	err := DB.Model(&Channel{}).Select("id, name, type, "+commonGroupCol).Where("status = ?", common.ChannelStatusEnabled).Order("id ASC").Find(&channels).Error
	return channels, err
}

func DeleteChannelPerfMetricsBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return DB.Where("bucket_ts < ?", cutoffTs).Delete(&ChannelPerfMetric{}).Error
}
