package controller

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type channelPreferenceAPIResponse struct {
	Success bool                      `json:"success"`
	Message string                    `json:"message"`
	Data    channelPreferenceResponse `json:"data"`
}

func setupChannelPreferenceTest(t *testing.T) (*gorm.DB, *model.User) {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalRedisEnabled := common.RedisEnabled
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()
	originalGroupRatio := ratio_setting.GroupRatio2JSONString()
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.RedisEnabled = originalRedisEnabled
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalGroupRatio))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
	})

	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1,"auto":1}`))
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","vip":"VIP","auto":"Auto"}`))
	t.Setenv("CHANNEL_ALIAS_KEY", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef")))
	require.NoError(t, service.InitChannelAlias())

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	user := &model.User{Username: "preference-user", Group: "default"}
	user.SetSetting(dto.UserSetting{NotifyType: dto.NotifyTypeEmail})
	require.NoError(t, db.Create(user).Error)
	return db, user
}

func callChannelPreferenceHandler(t *testing.T, userID int, method string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody []byte
	if body != nil {
		var err error
		requestBody, err = common.Marshal(body)
		require.NoError(t, err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, "/api/user/self/channel_preferences", bytes.NewReader(requestBody))
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Set("id", userID)
	common.SetContextKey(context, constant.ContextKeyUserGroup, "default")
	if method == http.MethodGet {
		GetChannelPreferences(context)
	} else {
		UpdateChannelPreferences(context)
	}
	return recorder
}

func TestGetChannelPreferencesOnlyReturnsEnabledChannelsAndNonAutoGroups(t *testing.T) {
	db, user := setupChannelPreferenceTest(t)
	enabledDefault := model.Channel{Name: "enabled-default", Group: "default", Status: common.ChannelStatusEnabled}
	disabledDefault := model.Channel{Name: "disabled-default", Group: "default", Status: common.ChannelStatusManuallyDisabled}
	enabledVIP := model.Channel{Name: "enabled-vip", Group: "vip", Status: common.ChannelStatusEnabled}
	enabledBoth := model.Channel{Name: "enabled-both", Group: "default,vip", Status: common.ChannelStatusEnabled}
	enabledAuto := model.Channel{Name: "enabled-auto", Group: "auto", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&[]*model.Channel{&enabledDefault, &disabledDefault, &enabledVIP, &enabledBoth, &enabledAuto}).Error)

	recorder := callChannelPreferenceHandler(t, user.Id, http.MethodGet, nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response channelPreferenceAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data.Groups, 2)
	assert.Equal(t, "default", response.Data.Groups[0].Group)
	assert.Equal(t, "vip", response.Data.Groups[1].Group)
	require.Len(t, response.Data.Groups[0].Channels, 2)
	require.Len(t, response.Data.Groups[1].Channels, 2)

	defaultIDs := map[int]struct{}{enabledDefault.Id: {}, enabledBoth.Id: {}}
	for _, option := range response.Data.Groups[0].Channels {
		channelID, err := service.DecryptChannelAlias("default", option.Alias)
		require.NoError(t, err)
		assert.Contains(t, defaultIDs, channelID)
		assert.Equal(t, "default-"+option.Alias, option.DisplayName)
	}
}

func TestUpdateChannelPreferencesValidatesAndPreservesOtherSettings(t *testing.T) {
	db, user := setupChannelPreferenceTest(t)
	enabledDefault := model.Channel{Name: "enabled-default", Group: "default", Status: common.ChannelStatusEnabled}
	disabledDefault := model.Channel{Name: "disabled-default", Group: "default", Status: common.ChannelStatusManuallyDisabled}
	enabledVIP := model.Channel{Name: "enabled-vip", Group: "vip", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&[]*model.Channel{&enabledDefault, &disabledDefault, &enabledVIP}).Error)
	defaultAlias, err := service.EncryptChannelAlias("default", enabledDefault.Id)
	require.NoError(t, err)
	disabledAlias, err := service.EncryptChannelAlias("default", disabledDefault.Id)
	require.NoError(t, err)
	crossGroupAlias, err := service.EncryptChannelAlias("default", enabledVIP.Id)
	require.NoError(t, err)
	autoAlias, err := service.EncryptChannelAlias("auto", enabledDefault.Id)
	require.NoError(t, err)

	invalidRequests := []map[string]string{
		{"default": disabledAlias},
		{"default": crossGroupAlias},
		{"internal": defaultAlias},
		{"auto": autoAlias},
	}
	for _, preferences := range invalidRequests {
		recorder := callChannelPreferenceHandler(t, user.Id, http.MethodPut, updateChannelPreferenceRequest{Preferences: preferences})
		var response channelPreferenceAPIResponse
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
		assert.False(t, response.Success)
	}

	recorder := callChannelPreferenceHandler(t, user.Id, http.MethodPut, updateChannelPreferenceRequest{
		Preferences: map[string]string{"default": defaultAlias, "vip": ""},
	})
	var response channelPreferenceAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)

	setting, err := model.GetUserSetting(user.Id, true)
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"default": enabledDefault.Id}, setting.PreferredChannels)
	assert.Equal(t, dto.NotifyTypeEmail, setting.NotifyType)
}

func TestAnonymizeUserLogsHidesRawChannelFields(t *testing.T) {
	_, _ = setupChannelPreferenceTest(t)
	logs := []*model.Log{{ChannelId: 17, Group: "default", ChannelName: "secret-upstream"}}

	responses, err := anonymizeUserLogs(logs)
	require.NoError(t, err)
	require.Len(t, responses, 1)
	assert.Regexp(t, `^default-[0-9A-Z]{6}$`, responses[0].ChannelDisplayName)

	payload, err := common.Marshal(responses[0])
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, common.Unmarshal(payload, &fields))
	assert.Equal(t, float64(0), fields["channel"])
	assert.Equal(t, "", fields["channel_name"])
}
