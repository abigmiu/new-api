package controller

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type channelPreferenceOption struct {
	Alias       string `json:"alias"`
	DisplayName string `json:"display_name"`
}

type channelPreferenceGroup struct {
	Group         string                    `json:"group"`
	SelectedAlias string                    `json:"selected_alias"`
	Channels      []channelPreferenceOption `json:"channels"`
}

type channelPreferenceResponse struct {
	Groups []channelPreferenceGroup `json:"groups"`
}

type updateChannelPreferenceRequest struct {
	Preferences map[string]string `json:"preferences"`
}

func GetChannelPreferences(c *gin.Context) {
	groups := getUserPreferenceGroups(common.GetContextKeyString(c, constant.ContextKeyUserGroup))
	channels, err := model.GetEnabledChannels()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setting, err := model.GetUserSetting(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	channelsByGroup := make(map[string][]channelPreferenceOption)
	for _, channel := range channels {
		for _, group := range channel.GetGroups() {
			if _, ok := groups[group]; !ok {
				continue
			}
			alias, aliasErr := service.EncryptChannelAlias(group, channel.Id)
			if aliasErr != nil {
				common.ApiError(c, aliasErr)
				return
			}
			channelsByGroup[group] = append(channelsByGroup[group], channelPreferenceOption{
				Alias:       alias,
				DisplayName: group + "-" + alias,
			})
		}
	}

	result := channelPreferenceResponse{Groups: make([]channelPreferenceGroup, 0, len(groups))}
	for group := range groups {
		options := channelsByGroup[group]
		if options == nil {
			options = []channelPreferenceOption{}
		}
		sort.Slice(options, func(i, j int) bool { return options[i].Alias < options[j].Alias })
		selectedAlias := ""
		if channelID, ok := setting.PreferredChannels[group]; ok {
			if alias, aliasErr := service.EncryptChannelAlias(group, channelID); aliasErr == nil {
				for _, option := range options {
					if option.Alias == alias {
						selectedAlias = alias
						break
					}
				}
			}
		}
		result.Groups = append(result.Groups, channelPreferenceGroup{Group: group, SelectedAlias: selectedAlias, Channels: options})
	}
	sort.Slice(result.Groups, func(i, j int) bool { return result.Groups[i].Group < result.Groups[j].Group })
	common.ApiSuccess(c, result)
}

func UpdateChannelPreferences(c *gin.Context) {
	var req updateChannelPreferenceRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}
	groups := getUserPreferenceGroups(common.GetContextKeyString(c, constant.ContextKeyUserGroup))
	channels, err := model.GetEnabledChannels()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channelsByID := make(map[int]*model.Channel, len(channels))
	for _, channel := range channels {
		channelsByID[channel.Id] = channel
	}

	preferences := make(map[string]int)
	for group, alias := range req.Preferences {
		if group == "auto" || strings.TrimSpace(group) == "" {
			common.ApiErrorI18n(c, i18n.MsgDistributorGroupAccessDenied)
			return
		}
		if _, ok := groups[group]; !ok {
			common.ApiErrorI18n(c, i18n.MsgDistributorGroupAccessDenied)
			return
		}
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		channelID, decryptErr := service.DecryptChannelAlias(group, alias)
		if decryptErr != nil {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		channel, ok := channelsByID[channelID]
		if !ok || !channelBelongsToGroup(channel, group) {
			common.ApiErrorI18n(c, i18n.MsgInvalidParams)
			return
		}
		preferences[group] = channelID
	}

	user, err := model.GetUserById(c.GetInt("id"), true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	setting := user.GetSetting()
	setting.PreferredChannels = preferences
	if err := model.UpdateUserSetting(user.Id, setting); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func getUserPreferenceGroups(userGroup string) map[string]struct{} {
	usable := service.GetUserUsableGroups(userGroup)
	groups := make(map[string]struct{})
	for group := range ratio_setting.GetGroupRatioCopy() {
		if group == "auto" {
			continue
		}
		if _, ok := usable[group]; ok {
			groups[group] = struct{}{}
		}
	}
	return groups
}

func channelBelongsToGroup(channel *model.Channel, group string) bool {
	for _, channelGroup := range channel.GetGroups() {
		if channelGroup == group {
			return true
		}
	}
	return false
}
