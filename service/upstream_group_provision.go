package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const upstreamGroupManagedTag = "upstream-ops-managed"

func upstreamKeyMatchesGroup(key UpstreamOpsKey, binding *model.UpstreamGroupBinding) bool {
	if strings.ToLower(strings.TrimSpace(key.Status)) != "active" || key.ModelLimitsEnabled {
		return false
	}
	if key.ExpiredTime > 0 && key.ExpiredTime <= common.GetTimestamp() {
		return false
	}
	if strings.TrimSpace(key.AllowIps) != "" || len(key.IpWhitelist) > 0 || len(key.IpBlacklist) > 0 {
		return false
	}
	if binding.RemoteGroupId != nil {
		return key.GroupId != nil && *key.GroupId == *binding.RemoteGroupId
	}
	return key.Group == binding.RemoteGroupName || key.GroupName == binding.RemoteGroupName
}

func ensureUpstreamGroupKey(ctx context.Context, client *UpstreamOpsClient, binding *model.UpstreamGroupBinding) (int64, string, error) {
	groups, err := client.ListKeyGroups(ctx, binding.UpstreamChannelId)
	if err != nil {
		return 0, "", err
	}
	foundGroup := false
	for _, group := range groups {
		if binding.RemoteGroupId != nil {
			foundGroup = group.Id != nil && *group.Id == *binding.RemoteGroupId
		} else {
			foundGroup = group.Name == binding.RemoteGroupName
		}
		if foundGroup {
			break
		}
	}
	if !foundGroup {
		return 0, "", errors.New("upstream group no longer exists")
	}

	eligible := make([]UpstreamOpsKey, 0)
	for page := 1; page <= 100; page++ {
		keyPage, listErr := client.ListKeys(ctx, binding.UpstreamChannelId, page)
		if listErr != nil {
			return 0, "", listErr
		}
		for _, key := range keyPage.Items {
			if upstreamKeyMatchesGroup(key, binding) {
				eligible = append(eligible, key)
			}
		}
		if keyPage.Pages <= page || len(keyPage.Items) == 0 {
			break
		}
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].Id < eligible[j].Id })
	var selected *UpstreamOpsKey
	if binding.UpstreamKeyId != nil {
		for i := range eligible {
			if eligible[i].Id == *binding.UpstreamKeyId {
				selected = &eligible[i]
				break
			}
		}
	}
	if selected == nil && len(eligible) > 0 {
		selected = &eligible[0]
	}
	if selected == nil {
		unlimited := true
		expiredTime := int64(-1)
		modelLimitsEnabled := false
		selected, err = client.CreateKey(ctx, binding.UpstreamChannelId, UpstreamOpsCreateKeyRequest{
			Name:               "new-api-managed-" + binding.LocalGroup,
			Group:              binding.RemoteGroupName,
			GroupId:            binding.RemoteGroupId,
			UnlimitedQuota:     &unlimited,
			ExpiredTime:        &expiredTime,
			ModelLimitsEnabled: &modelLimitsEnabled,
		})
		if err != nil {
			return 0, "", err
		}
	}
	key, err := client.RevealKey(ctx, binding.UpstreamChannelId, selected.Id)
	if err != nil {
		return 0, "", err
	}
	return selected.Id, key, nil
}

func validateManagedUpstreamURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return nil, errors.New("managed upstream URL must be HTTPS")
	}
	addresses, err := net.LookupIP(parsed.Hostname())
	if err != nil {
		return nil, errors.New("managed upstream host cannot be resolved")
	}
	for _, address := range addresses {
		if address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsUnspecified() {
			return nil, errors.New("managed upstream host resolves to a private address")
		}
	}
	return parsed, nil
}

func fetchManagedUpstreamModels(ctx context.Context, siteUrl string, key string) ([]string, error) {
	parsed, err := validateManagedUpstreamURL(siteUrl)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(parsed.String(), "/")+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New("managed upstream model request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("managed upstream models returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, upstreamOpsMaxResponseBytes+1))
	if err != nil || len(body) > upstreamOpsMaxResponseBytes {
		return nil, errors.New("managed upstream model response is invalid")
	}
	var envelope struct {
		Data []struct {
			Id string `json:"id"`
		} `json:"data"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil {
		return nil, errors.New("managed upstream model response is invalid")
	}
	seen := make(map[string]struct{}, len(envelope.Data))
	models := make([]string, 0, len(envelope.Data))
	for _, item := range envelope.Data {
		name := strings.TrimSpace(item.Id)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		models = append(models, name)
	}
	if len(models) == 0 {
		return nil, errors.New("managed upstream returned no models")
	}
	sort.Strings(models)
	return models, nil
}

func EnableUpstreamGroup(ctx context.Context, bindingId int64) (*model.UpstreamGroupBinding, error) {
	binding, err := model.GetUpstreamGroupBinding(bindingId)
	if err != nil {
		return nil, err
	}
	if binding.State == model.UpstreamGroupStateActive && binding.DesiredEnabled {
		return binding, nil
	}
	result := model.DB.Model(&model.UpstreamGroupBinding{}).Where("id = ? AND state != ?", binding.Id, model.UpstreamGroupStateProvisioning).Updates(map[string]any{
		"state":           model.UpstreamGroupStateProvisioning,
		"desired_enabled": true,
		"disabled_reason": "",
		"updated_time":    common.GetTimestamp(),
	})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		latest, getErr := model.GetUpstreamGroupBinding(binding.Id)
		if getErr == nil && latest.State == model.UpstreamGroupStateActive && latest.DesiredEnabled {
			return latest, nil
		}
		return nil, errors.New("upstream group provisioning is already in progress")
	}
	fail := func(state string, runErr error) (*model.UpstreamGroupBinding, error) {
		_ = model.DB.Model(&model.UpstreamGroupBinding{}).Where("id = ?", binding.Id).Updates(map[string]any{
			"state":           state,
			"disabled_reason": runErr.Error(),
			"updated_time":    common.GetTimestamp(),
		}).Error
		_ = model.DB.Create(&model.UpstreamGroupEvent{
			BindingId: binding.Id,
			EventType: "provisioning_failed",
			OldState:  model.UpstreamGroupStateProvisioning,
			NewState:  state,
			Message:   runErr.Error(),
		}).Error
		SendUpstreamGroupNotice(ctx, []string{fmt.Sprintf("%s / %s: provisioning failed (%s)", binding.UpstreamChannelName, binding.RemoteGroupName, runErr.Error())})
		return nil, runErr
	}
	client, err := NewUpstreamOpsClientFromEnv()
	if err != nil {
		return fail(model.UpstreamGroupStateError, err)
	}
	keyId, key, err := ensureUpstreamGroupKey(ctx, client, binding)
	if err != nil {
		return fail(model.UpstreamGroupStateKeyUnavailable, err)
	}
	models, err := fetchManagedUpstreamModels(ctx, binding.UpstreamSiteUrl, key)
	if err != nil {
		return fail(model.UpstreamGroupStateError, err)
	}
	channelType := constant.ChannelTypeNewAPI
	if binding.UpstreamChannelType == "sub2api" {
		channelType = constant.ChannelTypeSub2API
	}
	baseUrl := strings.TrimRight(binding.UpstreamSiteUrl, "/")
	tag := upstreamGroupManagedTag
	priority := int64(0)
	weight := uint(0)
	autoBan := 1
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		channel := model.Channel{
			Type:        channelType,
			Key:         key,
			Status:      common.ChannelStatusEnabled,
			Name:        binding.LocalDisplayName,
			Weight:      &weight,
			CreatedTime: common.GetTimestamp(),
			BaseURL:     &baseUrl,
			Models:      strings.Join(models, ","),
			Group:       binding.LocalGroup,
			Priority:    &priority,
			AutoBan:     &autoBan,
			Tag:         &tag,
		}
		if binding.LocalChannelId == nil {
			if err := tx.Create(&channel).Error; err != nil {
				return err
			}
		} else {
			channel.Id = *binding.LocalChannelId
			result := tx.Model(&model.Channel{}).Where("id = ? AND tag = ?", channel.Id, upstreamGroupManagedTag).
				Select("type", "key", "status", "name", "base_url", "models", "group", "priority", "weight", "auto_ban", "tag").Updates(&channel)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				channel.Id = 0
				if err := tx.Create(&channel).Error; err != nil {
					return err
				}
			} else if err := tx.Where("channel_id = ?", channel.Id).Delete(&model.Ability{}).Error; err != nil {
				return err
			}
		}
		if err := channel.AddAbilities(tx); err != nil {
			return err
		}
		return tx.Model(&model.UpstreamGroupBinding{}).Where("id = ?", binding.Id).Updates(map[string]any{
			"local_channel_id": channel.Id,
			"upstream_key_id":  keyId,
			"desired_enabled":  true,
			"state":            model.UpstreamGroupStateActive,
			"disabled_reason":  "",
			"updated_time":     common.GetTimestamp(),
		}).Error
	})
	if err != nil {
		return fail(model.UpstreamGroupStateError, err)
	}
	model.InitChannelCache()
	return model.GetUpstreamGroupBinding(binding.Id)
}

func DisableUpstreamGroup(bindingId int64) (*model.UpstreamGroupBinding, error) {
	binding, err := model.GetUpstreamGroupBinding(bindingId)
	if err != nil {
		return nil, err
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.UpstreamGroupBinding{}).Where("id = ?", binding.Id).Updates(map[string]any{
			"desired_enabled": false,
			"state":           model.UpstreamGroupStateAdminDisabled,
			"disabled_reason": "disabled by administrator",
			"updated_time":    common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		if binding.LocalChannelId == nil {
			return nil
		}
		if err := tx.Model(&model.Channel{}).Where("id = ?", *binding.LocalChannelId).Update("status", common.ChannelStatusManuallyDisabled).Error; err != nil {
			return err
		}
		return tx.Model(&model.Ability{}).Where("channel_id = ?", *binding.LocalChannelId).Update("enabled", false).Error
	})
	if err != nil {
		return nil, err
	}
	model.InitChannelCache()
	return model.GetUpstreamGroupBinding(binding.Id)
}

func ParseBindingId(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}
