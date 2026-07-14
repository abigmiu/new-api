package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRenameGroupsMigratesActiveReferencesAndSettings(t *testing.T) {
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalOptionMap := common.OptionMap
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalGroupGroupRatio := ratio_setting.GroupGroupRatio2JSONString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	originalAutoGroups := setting.AutoGroups2JsonString()
	originalTopupRatio := common.TopupGroupRatio2JSONString()
	originalRateLimits := setting.ModelRequestRateLimitGroup2JSONString()
	originalSpecialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.MarshalJSONString()
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.OptionMap = originalOptionMap
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalGroupGroupRatio))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(originalTopupRatio))
		require.NoError(t, setting.UpdateModelRequestRateLimitGroupByJSONString(originalRateLimits))
		require.NoError(t, types.LoadFromJsonString(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup, originalSpecialGroups))
	})

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	common.OptionMap = make(map[string]string)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Channel{},
		&model.Ability{},
		&model.Task{},
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.Option{},
	))

	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"legacy":1.2,"other":1}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"legacy":{"legacy":0.8,"other":0.9},"other":{"legacy":1.1}}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"legacy":"Legacy","other":"Other"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["legacy","other"]`))
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"legacy":1.5,"other":1}`))
	require.NoError(t, setting.UpdateModelRequestRateLimitGroupByJSONString(`{"legacy":[10,5]}`))
	require.NoError(t, types.LoadFromJsonString(
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup,
		`{"legacy":{"+:legacy":"visible","-:other":"hidden"},"other":{"legacy":"direct"}}`,
	))

	user := model.User{Username: "rename-user", Group: "legacy"}
	require.NoError(t, db.Create(&user).Error)
	token := model.Token{UserId: user.Id, Key: "rename-token", Name: "token", Group: "legacy"}
	require.NoError(t, db.Create(&token).Error)
	activeTask := model.Task{TaskID: "active-task", UserId: user.Id, Group: "legacy", Status: model.TaskStatusInProgress}
	require.NoError(t, db.Create(&activeTask).Error)
	completedTask := model.Task{TaskID: "completed-task", UserId: user.Id, Group: "legacy", Status: model.TaskStatusSuccess}
	require.NoError(t, db.Create(&completedTask).Error)
	channel := model.Channel{Name: "rename-channel", Group: "legacy,other", Models: "gpt-test", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "legacy", Model: "gpt-test", ChannelId: channel.Id, Enabled: true}).Error)
	plan := model.SubscriptionPlan{Title: "rename-plan", UpgradeGroup: "legacy", DowngradeGroup: "legacy"}
	require.NoError(t, db.Create(&plan).Error)
	activeSubscription := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", UpgradeGroup: "legacy", PrevUserGroup: "legacy", DowngradeGroup: "legacy"}
	require.NoError(t, db.Create(&activeSubscription).Error)
	expiredSubscription := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", UpgradeGroup: "legacy", PrevUserGroup: "legacy", DowngradeGroup: "legacy"}
	require.NoError(t, db.Create(&expiredSubscription).Error)

	require.NoError(t, RenameGroups(map[string]string{"legacy": "renamed"}))

	require.NoError(t, db.First(&user, user.Id).Error)
	assert.Equal(t, "renamed", user.Group)
	require.NoError(t, db.First(&token, token.Id).Error)
	assert.Equal(t, "renamed", token.Group)
	require.NoError(t, db.First(&activeTask, activeTask.ID).Error)
	assert.Equal(t, "renamed", activeTask.Group)
	require.NoError(t, db.First(&completedTask, completedTask.ID).Error)
	assert.Equal(t, "legacy", completedTask.Group)
	require.NoError(t, db.First(&channel, channel.Id).Error)
	assert.Equal(t, "renamed,other", channel.Group)
	var ability model.Ability
	require.NoError(t, db.Where("channel_id = ? AND `group` = ?", channel.Id, "renamed").First(&ability).Error)
	assert.Equal(t, "renamed", ability.Group)
	require.NoError(t, db.First(&plan, plan.Id).Error)
	assert.Equal(t, "renamed", plan.UpgradeGroup)
	assert.Equal(t, "renamed", plan.DowngradeGroup)
	require.NoError(t, db.First(&activeSubscription, activeSubscription.Id).Error)
	assert.Equal(t, "renamed", activeSubscription.UpgradeGroup)
	assert.Equal(t, "renamed", activeSubscription.PrevUserGroup)
	assert.Equal(t, "renamed", activeSubscription.DowngradeGroup)
	require.NoError(t, db.First(&expiredSubscription, expiredSubscription.Id).Error)
	assert.Equal(t, "legacy", expiredSubscription.UpgradeGroup)

	assert.Equal(t, map[string]float64{"renamed": 1.2, "other": 1}, ratio_setting.GetGroupRatioCopy())
	assert.Equal(t, []string{"renamed", "other"}, setting.GetAutoGroups())
	assert.Equal(t, "Legacy", setting.GetUsableGroupDescription("renamed"))
	assert.Equal(t, 1.5, common.GetTopupGroupRatio("renamed"))
	totalLimit, successLimit, found := setting.GetGroupRateLimit("renamed")
	assert.True(t, found)
	assert.Equal(t, 10, totalLimit)
	assert.Equal(t, 5, successLimit)
	ratio, found := ratio_setting.GetGroupGroupRatio("renamed", "renamed")
	assert.True(t, found)
	assert.Equal(t, 0.8, ratio)
	specialGroups, found := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get("renamed")
	assert.True(t, found)
	assert.Equal(t, "visible", specialGroups["+:renamed"])

	var optionCount int64
	require.NoError(t, db.Model(&model.Option{}).Where("`key` IN ?", []string{
		"GroupRatio",
		"UserUsableGroups",
		"GroupGroupRatio",
		"group_ratio_setting.group_special_usable_group",
		"AutoGroups",
		"TopupGroupRatio",
		"ModelRequestRateLimitGroup",
	}).Count(&optionCount).Error)
	assert.EqualValues(t, 7, optionCount)
}
