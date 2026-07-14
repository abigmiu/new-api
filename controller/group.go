package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

type GroupRenameItem struct {
	OldName string `json:"old_name"`
	NewName string `json:"new_name"`
}

type GroupRenameRequest struct {
	Renames []GroupRenameItem `json:"renames"`
}

func RenameGroups(c *gin.Context) {
	var request GroupRenameRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || len(request.Renames) == 0 {
		common.ApiErrorMsg(c, "无效的分组重命名参数")
		return
	}
	renames := make(map[string]string, len(request.Renames))
	for _, item := range request.Renames {
		oldName := strings.TrimSpace(item.OldName)
		if _, ok := renames[oldName]; ok {
			common.ApiErrorMsg(c, "旧分组名称重复")
			return
		}
		renames[oldName] = strings.TrimSpace(item.NewName)
	}
	if err := service.RenameGroups(renames); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	recordManageAudit(c, "group.rename", map[string]interface{}{
		"renames": request.Renames,
	})
	common.ApiSuccess(c, nil)
}

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]interface{})
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	for groupName, _ := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]interface{}{
				"ratio": service.GetUserGroupRatio(userGroup, groupName),
				"desc":  desc,
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio": "自动",
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
