package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func renameMapKeys[V any](source map[string]V, renames map[string]string) map[string]V {
	result := make(map[string]V, len(source))
	for key, value := range source {
		if newKey, ok := renames[key]; ok {
			key = newKey
		}
		result[key] = value
	}
	return result
}

func buildGroupRenameOptionValues(renames map[string]string) (map[string]string, error) {
	groupRatio := renameMapKeys(ratio_setting.GetGroupRatioCopy(), renames)
	userUsableGroups := renameMapKeys(setting.GetUserUsableGroupsCopy(), renames)

	groupGroupRatio := make(map[string]map[string]float64)
	for userGroup, ratios := range ratio_setting.GetGroupRatioSetting().GroupGroupRatio.ReadAll() {
		if newName, ok := renames[userGroup]; ok {
			userGroup = newName
		}
		groupGroupRatio[userGroup] = renameMapKeys(ratios, renames)
	}

	groupSpecialUsableGroup := make(map[string]map[string]string)
	for userGroup, groups := range ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll() {
		if newName, ok := renames[userGroup]; ok {
			userGroup = newName
		}
		renamedGroups := make(map[string]string, len(groups))
		for group, description := range groups {
			prefix := ""
			name := group
			if strings.HasPrefix(group, "+:") || strings.HasPrefix(group, "-:") {
				prefix = group[:2]
				name = group[2:]
			}
			if newName, ok := renames[name]; ok {
				name = newName
			}
			renamedGroups[prefix+name] = description
		}
		groupSpecialUsableGroup[userGroup] = renamedGroups
	}

	autoGroups := setting.GetAutoGroups()
	renamedAutoGroups := make([]string, len(autoGroups))
	for i, group := range autoGroups {
		if newName, ok := renames[group]; ok {
			group = newName
		}
		renamedAutoGroups[i] = group
	}

	var topupGroupRatio map[string]float64
	if err := common.UnmarshalJsonStr(common.TopupGroupRatio2JSONString(), &topupGroupRatio); err != nil {
		return nil, err
	}
	var requestRateLimit map[string][2]int
	if err := common.UnmarshalJsonStr(setting.ModelRequestRateLimitGroup2JSONString(), &requestRateLimit); err != nil {
		return nil, err
	}

	values := map[string]any{
		"GroupRatio":       groupRatio,
		"UserUsableGroups": userUsableGroups,
		"GroupGroupRatio":  groupGroupRatio,
		"group_ratio_setting.group_special_usable_group": groupSpecialUsableGroup,
		"AutoGroups":                 renamedAutoGroups,
		"TopupGroupRatio":            renameMapKeys(topupGroupRatio, renames),
		"ModelRequestRateLimitGroup": renameMapKeys(requestRateLimit, renames),
	}
	optionValues := make(map[string]string, len(values))
	for key, value := range values {
		data, err := common.Marshal(value)
		if err != nil {
			return nil, err
		}
		optionValues[key] = string(data)
	}
	return optionValues, nil
}

func RenameGroups(renames map[string]string) error {
	if len(renames) == 0 {
		return errors.New("至少需要一个分组重命名")
	}
	currentGroups := ratio_setting.GetGroupRatioCopy()
	newNames := make(map[string]struct{}, len(renames))
	for oldName, newName := range renames {
		oldName = strings.TrimSpace(oldName)
		newName = strings.TrimSpace(newName)
		if oldName == "" || newName == "" || oldName == newName {
			return errors.New("分组名称不能为空且新旧名称不能相同")
		}
		if oldName == "default" || oldName == "auto" || newName == "auto" {
			return errors.New("default 和 auto 分组不能重命名")
		}
		if len(newName) > 64 || strings.Contains(newName, ",") {
			return errors.New("新分组名称不能超过 64 个字符或包含逗号")
		}
		if _, ok := currentGroups[oldName]; !ok {
			return fmt.Errorf("分组 %s 不存在", oldName)
		}
		if _, ok := currentGroups[newName]; ok {
			return fmt.Errorf("分组 %s 已存在", newName)
		}
		if _, ok := newNames[newName]; ok {
			return fmt.Errorf("新分组名称 %s 重复", newName)
		}
		newNames[newName] = struct{}{}
	}

	optionValues, err := buildGroupRenameOptionValues(renames)
	if err != nil {
		return err
	}
	if _, err = model.RenameGroupReferences(renames, optionValues); err != nil {
		return err
	}
	ClearChannelAffinityCacheAll()
	return nil
}
