package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

func RenameGroupReferences(renames map[string]string, optionValues map[string]string) ([]int, error) {
	userIds := make([]int, 0)
	err := DB.Transaction(func(tx *gorm.DB) error {
		for oldName, newName := range renames {
			var ids []int
			if err := tx.Model(&User{}).Where(&User{Group: oldName}).Pluck("id", &ids).Error; err != nil {
				return err
			}
			userIds = append(userIds, ids...)
			if err := tx.Model(&User{}).Where(&User{Group: oldName}).Update("group", newName).Error; err != nil {
				return err
			}
			if err := tx.Model(&Token{}).Where(&Token{Group: oldName}).Update("group", newName).Error; err != nil {
				return err
			}
			if err := tx.Model(&Task{}).
				Where(&Task{Group: oldName}).
				Where("status NOT IN ?", []TaskStatus{TaskStatusFailure, TaskStatusSuccess}).
				Update("group", newName).Error; err != nil {
				return err
			}
			if err := tx.Model(&SubscriptionPlan{}).Where("upgrade_group = ?", oldName).Update("upgrade_group", newName).Error; err != nil {
				return err
			}
			if err := tx.Model(&SubscriptionPlan{}).Where("downgrade_group = ?", oldName).Update("downgrade_group", newName).Error; err != nil {
				return err
			}
			for _, column := range []string{"upgrade_group", "prev_user_group", "downgrade_group"} {
				if err := tx.Model(&UserSubscription{}).Where("status = ?", "active").Where(column+" = ?", oldName).Update(column, newName).Error; err != nil {
					return err
				}
			}
		}

		var channels []Channel
		if err := tx.Find(&channels).Error; err != nil {
			return err
		}
		for i := range channels {
			groups := strings.Split(channels[i].Group, ",")
			changed := false
			seen := make(map[string]struct{}, len(groups))
			renamedGroups := make([]string, 0, len(groups))
			for _, group := range groups {
				if newName, ok := renames[group]; ok {
					group = newName
					changed = true
				}
				if _, ok := seen[group]; ok {
					continue
				}
				seen[group] = struct{}{}
				renamedGroups = append(renamedGroups, group)
			}
			if !changed {
				continue
			}
			channels[i].Group = strings.Join(renamedGroups, ",")
			if err := tx.Model(&Channel{}).Where("id = ?", channels[i].Id).Update("group", channels[i].Group).Error; err != nil {
				return err
			}
			if err := channels[i].UpdateAbilities(tx); err != nil {
				return err
			}
		}

		for key, value := range optionValues {
			option := Option{Key: key}
			if err := tx.FirstOrCreate(&option, Option{Key: key}).Error; err != nil {
				return err
			}
			option.Value = value
			if err := tx.Save(&option).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for key, value := range optionValues {
		if err := updateOptionMap(key, value); err != nil {
			return nil, err
		}
	}
	for _, userId := range userIds {
		if err := InvalidateUserCache(userId); err != nil {
			common.SysError("failed to invalidate renamed group user cache: " + err.Error())
		}
	}
	InitChannelCache()
	return userIds, nil
}
