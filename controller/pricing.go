package controller

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// managedGroupOption mirrors the shape returned by GetTokenGroups so the model
// square can show a managed group's display name instead of its raw key.
type managedGroupOption struct {
	Value        string `json:"value"`
	Label        string `json:"label"`
	SaleRatio    string `json:"sale_ratio"`
	PriceVersion int64  `json:"price_version"`
}

func GetPricing(c *gin.Context) {
	pricing := model.GetPricing()
	userId, exists := c.Get("id")
	usableGroup := map[string]string{}
	groupRatio := map[string]float64{}
	for s, f := range ratio_setting.GetGroupRatioCopy() {
		groupRatio[s] = f
	}
	var group string
	if exists {
		user, err := model.GetUserCache(userId.(int))
		if err == nil {
			group = user.Group
			for g := range groupRatio {
				ratio, ok := ratio_setting.GetGroupGroupRatio(group, g)
				if ok {
					groupRatio[g] = ratio
				}
			}
		}
	}

	usableGroup = service.GetUserUsableGroups(group)

	// 托管分组未注册进 GroupRatio/UserUsableGroups，其倍率只存在于活跃绑定上。
	// 价目表需要展示全量分组，因此把这些倍率并入返回的 groupRatio 副本。
	managedGroups := make([]managedGroupOption, 0)
	bindings, err := model.ListActiveUpstreamGroupBindings()
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("获取托管分组失败，价目表将缺失托管分组倍率: %v", err))
	}
	for _, binding := range bindings {
		saleRatio, parseErr := strconv.ParseFloat(binding.SaleRatio, 64)
		if parseErr != nil || saleRatio <= 0 {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("跳过倍率无效的托管分组 group=%s sale_ratio=%q error=%v", binding.LocalGroup, binding.SaleRatio, parseErr))
			continue
		}
		groupRatio[binding.LocalGroup] = saleRatio
		managedGroups = append(managedGroups, managedGroupOption{
			Value:        binding.LocalGroup,
			Label:        binding.LocalDisplayName,
			SaleRatio:    binding.SaleRatio,
			PriceVersion: binding.PriceVersion,
		})
	}

	c.JSON(200, gin.H{
		"success":            true,
		"data":               pricing,
		"vendors":            model.GetVendors(),
		"group_ratio":        groupRatio,
		"usable_group":       usableGroup,
		"managed_groups":     managedGroups,
		"supported_endpoint": model.GetSupportedEndpointMap(),
		"auto_groups":        service.GetUserAutoGroup(group),
		"pricing_version":    "a42d372ccf0b5dd13ecf71203521f9d2",
	})
}

func ResetModelRatio(c *gin.Context) {
	defaultStr := ratio_setting.DefaultModelRatio2JSONString()
	err := model.UpdateOption("ModelRatio", defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	err = ratio_setting.UpdateModelRatioByJSONString(defaultStr)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(200, gin.H{
		"success": true,
		"message": "重置模型倍率成功",
	})
}
