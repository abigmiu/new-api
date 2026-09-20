package controller

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func ListUpstreamGroups(c *gin.Context) {
	bindings, err := model.ListUpstreamGroupBindings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	bindingIds := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		bindingIds = append(bindingIds, binding.Id)
	}
	affectedCounts, err := model.CountAffectedTokenGroups(bindingIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	type upstreamGroupView struct {
		Id                  int64  `json:"id"`
		UpstreamChannelId   int64  `json:"upstream_channel_id"`
		UpstreamChannelName string `json:"upstream_channel_name"`
		UpstreamChannelType string `json:"upstream_channel_type"`
		RemoteGroupName     string `json:"remote_group_name"`
		LocalGroup          string `json:"local_group"`
		LocalDisplayName    string `json:"local_display_name"`
		LocalChannelId      *int   `json:"local_channel_id,omitempty"`
		UpstreamKeyReady    bool   `json:"upstream_key_ready"`
		DesiredEnabled      bool   `json:"desired_enabled"`
		State               string `json:"state"`
		SourceRatio         string `json:"source_ratio"`
		SaleRatio           string `json:"sale_ratio"`
		PriceVersion        int64  `json:"price_version"`
		AffectedKeyCount    int64  `json:"affected_key_count"`
		DisabledReason      string `json:"disabled_reason,omitempty"`
		LastSyncedAt        int64  `json:"last_synced_at"`
	}
	views := make([]upstreamGroupView, 0, len(bindings))
	for _, binding := range bindings {
		views = append(views, upstreamGroupView{
			Id:                  binding.Id,
			UpstreamChannelId:   binding.UpstreamChannelId,
			UpstreamChannelName: binding.UpstreamChannelName,
			UpstreamChannelType: binding.UpstreamChannelType,
			RemoteGroupName:     binding.RemoteGroupName,
			LocalGroup:          binding.LocalGroup,
			LocalDisplayName:    binding.LocalDisplayName,
			LocalChannelId:      binding.LocalChannelId,
			UpstreamKeyReady:    binding.UpstreamKeyId != nil,
			DesiredEnabled:      binding.DesiredEnabled,
			State:               binding.State,
			SourceRatio:         binding.SourceRatio,
			SaleRatio:           binding.SaleRatio,
			PriceVersion:        binding.PriceVersion,
			AffectedKeyCount:    affectedCounts[binding.Id],
			DisabledReason:      binding.DisabledReason,
			LastSyncedAt:        binding.LastSyncedAt,
		})
	}
	common.ApiSuccess(c, views)
}

func SyncUpstreamGroups(c *gin.Context) {
	task, created, err := service.EnqueueSystemTask(model.SystemTaskTypeUpstreamGroupSync, map[string]bool{"manual": true})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"task": task.ToResponse(), "created": created})
}

func EnableUpstreamGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	binding, err := service.EnableUpstreamGroup(ctx, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func DisableUpstreamGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	binding, err := service.DisableUpstreamGroup(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func ListUpstreamGroupEvents(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	events, err := model.ListUpstreamGroupEvents(id, limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": events})
}
