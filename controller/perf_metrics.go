package controller

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

func GetPerfMetricsSummary(c *gin.Context) {
	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	activeGroups := append(lo.Keys(ratio_setting.GetGroupRatioCopy()), "auto")
	result, err := perfmetrics.QuerySummaryAll(hours, activeGroups)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func GetPerfMetrics(c *gin.Context) {
	modelName := c.Query("model")
	if modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "model is required",
		})
		return
	}

	hours := 24
	if rawHours := c.Query("hours"); rawHours != "" {
		if parsed, err := strconv.Atoi(rawHours); err == nil {
			hours = parsed
		}
	}

	result, err := perfmetrics.Query(perfmetrics.QueryParams{
		Model: modelName,
		Group: c.Query("group"),
		Hours: hours,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	result.Groups = filterActiveGroups(result.Groups)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func filterActiveGroups(groups []perfmetrics.GroupResult) []perfmetrics.GroupResult {
	activeRatios := ratio_setting.GetGroupRatioCopy()
	return lo.Filter(groups, func(g perfmetrics.GroupResult, _ int) bool {
		_, ok := activeRatios[g.Group]
		return ok || g.Group == "auto"
	})
}

func GetChannelPerformance(c *gin.Context) {
	hours := 24
	selectedRange := "24h"
	switch c.Query("range") {
	case "1h":
		hours = 1
		selectedRange = "1h"
	case "7d":
		hours = 24 * 7
		selectedRange = "7d"
	}
	result, err := perfmetrics.QueryChannelPerformance(hours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	bindings, err := model.ListActiveUpstreamGroupBindings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	response := prepareManagedChannelPerformanceResponse(result, bindings)
	response.Range = selectedRange
	c.JSON(http.StatusOK, gin.H{"success": true, "data": response})
}

type managedChannelPerformanceResponse struct {
	UpdatedAt int64                            `json:"updated_at"`
	Range     string                           `json:"range"`
	Suppliers []managedChannelPerformanceGroup `json:"suppliers"`
}

type managedChannelPerformanceGroup struct {
	UpstreamChannelId   int64                     `json:"upstream_channel_id"`
	UpstreamChannelName string                    `json:"upstream_channel_name"`
	Groups              []managedGroupPerformance `json:"groups"`
}

type managedGroupPerformance struct {
	BindingId    int64                                  `json:"binding_id"`
	GroupName    string                                 `json:"group_name"`
	Description  string                                 `json:"description"`
	SaleRatio    string                                 `json:"sale_ratio"`
	AttemptCount int64                                  `json:"attempt_count"`
	SuccessCount int64                                  `json:"success_count"`
	SuccessRate  float64                                `json:"success_rate"`
	AvgLatencyMs int64                                  `json:"avg_latency_ms"`
	AvgTtftMs    int64                                  `json:"avg_ttft_ms"`
	AvgTps       float64                                `json:"avg_tps"`
	CacheHitRate *float64                               `json:"cache_hit_rate"`
	CacheRate    *float64                               `json:"cache_rate"`
	Series       []perfmetrics.ChannelPerformanceBucket `json:"series"`
}

func prepareManagedChannelPerformanceResponse(result perfmetrics.ChannelPerformanceResult, bindings []model.UpstreamGroupBinding) managedChannelPerformanceResponse {
	metricsByChannel := make(map[int]perfmetrics.ChannelPerformance, len(result.Channels))
	for _, metric := range result.Channels {
		metricsByChannel[metric.ChannelID] = metric
	}
	supplierIndex := make(map[int64]int, len(bindings))
	response := managedChannelPerformanceResponse{UpdatedAt: result.UpdatedAt, Suppliers: make([]managedChannelPerformanceGroup, 0)}
	for _, binding := range bindings {
		if binding.LocalChannelId == nil {
			continue
		}
		index, exists := supplierIndex[binding.UpstreamChannelId]
		if !exists {
			index = len(response.Suppliers)
			supplierIndex[binding.UpstreamChannelId] = index
			response.Suppliers = append(response.Suppliers, managedChannelPerformanceGroup{
				UpstreamChannelId:   binding.UpstreamChannelId,
				UpstreamChannelName: binding.UpstreamChannelName,
				Groups:              make([]managedGroupPerformance, 0),
			})
		}
		metric := metricsByChannel[*binding.LocalChannelId]
		series := metric.Series
		if series == nil {
			series = make([]perfmetrics.ChannelPerformanceBucket, 0)
		}
		response.Suppliers[index].Groups = append(response.Suppliers[index].Groups, managedGroupPerformance{
			BindingId: binding.Id, GroupName: binding.RemoteGroupName, Description: binding.RemoteDescription,
			SaleRatio: binding.SaleRatio, AttemptCount: metric.AttemptCount, SuccessCount: metric.SuccessCount,
			SuccessRate: metric.SuccessRate, AvgLatencyMs: metric.AvgLatencyMs, AvgTtftMs: metric.AvgTtftMs,
			AvgTps: metric.AvgTps, CacheHitRate: metric.CacheHitRate, CacheRate: metric.CacheRate, Series: series,
		})
	}
	sort.Slice(response.Suppliers, func(i, j int) bool {
		if response.Suppliers[i].UpstreamChannelName == response.Suppliers[j].UpstreamChannelName {
			return response.Suppliers[i].UpstreamChannelId < response.Suppliers[j].UpstreamChannelId
		}
		return response.Suppliers[i].UpstreamChannelName < response.Suppliers[j].UpstreamChannelName
	})
	for i := range response.Suppliers {
		sort.Slice(response.Suppliers[i].Groups, func(a, b int) bool {
			if response.Suppliers[i].Groups[a].GroupName == response.Suppliers[i].Groups[b].GroupName {
				return response.Suppliers[i].Groups[a].BindingId < response.Suppliers[i].Groups[b].BindingId
			}
			return response.Suppliers[i].Groups[a].GroupName < response.Suppliers[i].Groups[b].GroupName
		})
	}
	return response
}
