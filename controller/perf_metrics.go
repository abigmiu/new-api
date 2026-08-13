package controller

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/service"
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
	group := strings.TrimSpace(c.Query("group"))
	if group == "" {
		group = "gpt-0.1倍率"
	}
	isAdmin := c.GetInt("role") >= common.RoleAdminUser
	groups := getChannelPerformanceGroups(common.GetContextKeyString(c, constant.ContextKeyUserGroup), isAdmin)
	if _, ok := groups[group]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid group"})
		return
	}
	hours := 24
	switch c.Query("range") {
	case "1h":
		hours = 1
	case "7d":
		hours = 24 * 7
	}
	result, err := perfmetrics.QueryChannelPerformance(hours)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := prepareChannelPerformanceResponse(&result, group, groups, isAdmin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func getChannelPerformanceGroups(userGroup string, isAdmin bool) map[string]struct{} {
	if !isAdmin {
		return getUserPreferenceGroups(userGroup)
	}
	groups := make(map[string]struct{})
	for group := range ratio_setting.GetGroupRatioCopy() {
		if group != "auto" {
			groups[group] = struct{}{}
		}
	}
	return groups
}

func prepareChannelPerformanceResponse(result *perfmetrics.ChannelPerformanceResult, group string, groups map[string]struct{}, isAdmin bool) error {
	result.Groups = make([]string, 0, len(groups))
	for name := range groups {
		result.Groups = append(result.Groups, name)
	}
	sort.Strings(result.Groups)
	result.SelectedGroup = group
	result.IsAdmin = isAdmin

	channels := result.Channels[:0]
	for _, channel := range result.Channels {
		if !lo.Contains(channel.Groups, group) {
			continue
		}
		alias, err := service.EncryptChannelAlias(group, channel.ChannelID)
		if err != nil {
			return err
		}
		channel.Alias = alias
		if isAdmin {
			channel.DisplayName = channel.ChannelName
		} else {
			channel.DisplayName = group + "-" + alias
			channel.ChannelID = 0
			channel.ChannelName = ""
		}
		channels = append(channels, channel)
	}
	result.Channels = channels
	return nil
}
